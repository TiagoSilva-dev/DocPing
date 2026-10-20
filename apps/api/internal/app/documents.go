package app

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"docping/api/internal/domain"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func safeFilename(v string) string {
	v = strings.ReplaceAll(v, "\\", "/")
	v = filepath.Base(v)
	v = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune("<>:\"|?*", r) {
			return -1
		}
		return r
	}, v)
	v = strings.Trim(v, ". ")
	if len(v) > 180 {
		v = string([]rune(v)[:min(100, len([]rune(v)))])
	}
	if v == "" {
		v = "documento"
	}
	return v
}
func detectFile(name string, data []byte) (string, error) {
	kind := http.DetectContentType(data)
	ext := strings.ToLower(filepath.Ext(name))
	valid := false
	switch kind {
	case "application/pdf":
		valid = ext == ".pdf"
	case "image/jpeg":
		valid = ext == ".jpg" || ext == ".jpeg"
	case "image/png":
		valid = ext == ".png"
	}
	if len(data) >= 12 && string(data[4:8]) == "ftyp" {
		brand := string(data[8:12])
		if brand == "heic" || brand == "heix" || brand == "hevc" || brand == "hevx" {
			kind = "image/heic"
			valid = ext == ".heic"
		}
	}
	if !valid {
		return "", invalid("Envie um PDF, JPG, PNG ou HEIC com extensão e conteúdo válidos.")
	}
	return kind, nil
}

// Reading the mobile connection, malware scanning and object I/O deliberately
// happen outside the transaction. Commit rechecks identity, state, duplicates
// and quotas so revocation/review during transfer cannot bypass authorization.
func (a *App) uploadHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	var initial domain.Request
	err := a.Store.Run(ctx, false, func(s *domain.State) error {
		c := &call{A: a, W: w, R: r, S: s}
		_, req, e := c.publicToken(true)
		if e != nil {
			return e
		}
		initial = req
		if req.Status == "completed" {
			return fail(409, "request_completed", "Os documentos já foram aprovados. O envio foi encerrado.")
		}
		for _, item := range req.Items {
			if item.ID == r.PathValue("itemID") {
				if item.Status == "approved" {
					return fail(409, "item_approved", "Este documento já foi aprovado.")
				}
				return nil
			}
		}
		return missing()
	})
	if err != nil {
		writeError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.Config.MaxUpload+(1<<20))
	if err = r.ParseMultipartForm(1 << 20); err != nil {
		writeError(w, fail(413, "upload_too_large", "O arquivo excede o limite de tamanho permitido."))
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, invalid("Escolha um arquivo para enviar."))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, a.Config.MaxUpload+1))
	if err != nil {
		writeError(w, invalid("Não foi possível ler o arquivo."))
		return
	}
	if len(data) == 0 || int64(len(data)) > a.Config.MaxUpload {
		writeError(w, fail(413, "upload_too_large", "O arquivo está vazio ou excede o limite de tamanho."))
		return
	}
	name := safeFilename(header.Filename)
	kind, err := detectFile(name, data)
	if err != nil {
		writeError(w, err)
		return
	}
	scan, err := a.Objects.Scan(ctx, data)
	if err != nil {
		status, code := 503, "scan_failed"
		if scan == "rejected" {
			status, code = 422, "malware_rejected"
		}
		writeError(w, fail(status, code, err.Error()))
		return
	}
	sum := sha256.Sum256(data)
	prepared := domain.DocumentFile{ID: domain.ID(), Name: name, MIMEType: kind, Size: int64(len(data)), CreatedAt: domain.Now(), ScanStatus: scan, SHA256: hex.EncodeToString(sum[:])}
	prepared.ObjectKey = initial.OrgID + "/" + initial.ID + "/" + prepared.ID
	if err = a.Objects.Put(ctx, prepared.ObjectKey, kind, data); err != nil {
		writeError(w, fail(503, "storage_unavailable", "Não foi possível salvar o arquivo. Tente novamente."))
		return
	}
	consumed := false
	var result any
	err = a.Store.Run(ctx, true, func(s *domain.State) error {
		c := &call{A: a, W: w, R: r, S: s}
		var e error
		result, e = a.commitUpload(c, prepared, &consumed)
		return e
	})
	if err != nil || !consumed {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = a.Objects.Delete(cleanup, prepared.ObjectKey)
		cancel()
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, 200, result)
}
func (a *App) commitUpload(c *call, prepared domain.DocumentFile, consumed *bool) (any, error) {
	_, r, err := c.publicToken(true)
	if err != nil {
		return nil, err
	}
	if r.Status == "completed" {
		return nil, fail(409, "request_completed", "Os documentos já foram aprovados. O envio foi encerrado.")
	}
	index := -1
	for n, item := range r.Items {
		if item.ID == c.R.PathValue("itemID") {
			index = n
			break
		}
	}
	if index < 0 {
		return nil, missing()
	}
	item := &r.Items[index]
	if item.Status == "approved" {
		return nil, fail(409, "item_approved", "Este documento já foi aprovado.")
	}
	f := prepared
	digest := f.SHA256
	active := 0
	for _, f := range item.Files {
		if !f.Superseded {
			active++
			if f.SHA256 == digest && item.Status != "correction_required" {
				return domain.PublicRequest(r), nil
			}
		}
	}
	if item.Status != "correction_required" && active >= item.MaxFiles {
		return nil, invalid("O limite de arquivos deste documento foi atingido.")
	}
	orgTotal, requestTotal := int64(0), int64(0)
	for _, existing := range c.S.Requests {
		if existing.OrgID != r.OrgID {
			continue
		}
		for _, i := range existing.Items {
			for _, f := range i.Files {
				orgTotal += f.Size
				if existing.ID == r.ID {
					requestTotal += f.Size
				}
			}
		}
	}
	orgLimit, requestLimit := a.Config.MaxOrgBytes, a.Config.MaxRequestBytes
	if orgLimit == 0 {
		orgLimit = 1 << 30
	}
	if requestLimit == 0 {
		requestLimit = 500 << 20
	}
	if orgTotal+f.Size > orgLimit || requestTotal+f.Size > requestLimit {
		return nil, fail(422, "storage_limit", "O limite de armazenamento foi atingido. Contate o escritório.")
	}
	if item.Status == "correction_required" {
		target := item.CorrectionFileID
		if target == "" && active == 1 {
			for _, f := range item.Files {
				if !f.Superseded {
					target = f.ID
				}
			}
		}
		replaced := false
		for n := range item.Files {
			if item.Files[n].ID == target && !item.Files[n].Superseded {
				item.Files[n].Superseded = true
				replaced = true
			}
		}
		if !replaced {
			return nil, fail(409, "correction_target_missing", "O escritório precisa identificar o arquivo a corrigir.")
		}
	}
	*consumed = true
	item.Files = append(item.Files, f)
	item.Status = "uploaded"
	item.CorrectionReason = ""
	item.CorrectionFileID = ""
	r.UpdatedAt = domain.Now()
	domain.Recalculate(&r)
	c.S.Requests[r.ID] = r
	c.S.Audit(r.OrgID, "", "document.uploaded", f.ID)
	return domain.PublicRequest(r), nil
}
func (a *App) authorizedFile(ctx context.Context, r *http.Request) (domain.DocumentFile, error) {
	var found domain.DocumentFile
	err := a.Store.Run(ctx, false, func(s *domain.State) error {
		c := &call{A: a, R: r, S: s}
		orgID, requestID := "", ""
		if r.URL.Query().Get("token") != "" {
			_, req, e := c.publicToken(true)
			if e != nil {
				return e
			}
			orgID = req.OrgID
			requestID = req.ID
		} else {
			if e := c.authenticate(false); e != nil {
				return e
			}
			orgID = c.Org.ID
		}
		for _, req := range s.Requests {
			if req.OrgID != orgID || requestID != "" && req.ID != requestID {
				continue
			}
			for _, item := range req.Items {
				for _, f := range item.Files {
					if f.ID == r.PathValue("id") {
						if f.ScanStatus != "clean" && !f.Sample && !(a.Config.Env == "development" && f.ScanStatus == "unscanned_development") {
							return fail(423, "file_quarantined", "Arquivo indisponível para download.")
						}
						found = f
						return nil
					}
				}
			}
		}
		return missing()
	})
	return found, err
}
func (a *App) file(w http.ResponseWriter, r *http.Request) {
	f, err := a.authorizedFile(r.Context(), r)
	if err != nil {
		writeError(w, err)
		return
	}
	var reader io.ReadCloser
	if f.Sample {
		reader = io.NopCloser(bytes.NewReader(samplePDF()))
	} else {
		reader, err = a.Objects.Get(r.Context(), f.ObjectKey)
	}
	if err != nil {
		writeError(w, fail(404, "file_missing", "Arquivo indisponível."))
		return
	}
	defer reader.Close()
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'self' "+a.Config.Origin)
	w.Header().Set("Content-Type", f.MIMEType)
	disposition := "inline"
	if f.MIMEType == "image/heic" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": f.Name}))
	w.Header().Set("Content-Length", fmt.Sprint(f.Size))
	w.WriteHeader(200)
	_, _ = io.Copy(w, reader)
}
func (a *App) export(w http.ResponseWriter, r *http.Request) {
	var req domain.Request
	err := a.Store.Run(r.Context(), false, func(s *domain.State) error {
		c := &call{A: a, W: w, R: r, S: s}
		if e := c.authenticate(false); e != nil {
			return e
		}
		var e error
		req, e = c.ownRequest(r.PathValue("id"))
		return e
	})
	if err != nil {
		writeError(w, err)
		return
	}
	type entry struct {
		file   domain.DocumentFile
		path   string
		reader io.ReadCloser
	}
	entries := []entry{}
	defer func() {
		for _, e := range entries {
			e.reader.Close()
		}
	}()
	for _, item := range req.Items {
		for _, f := range item.Files {
			if f.Superseded || !(f.ScanStatus == "clean" || f.Sample || a.Config.Env == "development" && f.ScanStatus == "unscanned_development") {
				continue
			}
			var reader io.ReadCloser
			var e error
			if f.Sample {
				reader = io.NopCloser(bytes.NewReader(samplePDF()))
			} else {
				reader, e = a.Objects.Get(r.Context(), f.ObjectKey)
			}
			if e != nil {
				writeError(w, fail(503, "export_unavailable", "Um arquivo não está disponível. Tente novamente."))
				return
			}
			entries = append(entries, entry{file: f, path: safeFilename(item.Name) + "/" + f.ID[:8] + "-" + safeFilename(f.Name), reader: reader})
		}
	}
	if len(entries) == 0 {
		writeError(w, fail(422, "no_files", "Não há arquivos disponíveis para exportar."))
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": safeFilename(req.ClientName) + "-" + req.Competence + ".zip"}))
	writer := zip.NewWriter(w)
	for _, e := range entries {
		dst, err := writer.Create(e.path)
		if err != nil {
			break
		}
		if _, err = io.Copy(dst, e.reader); err != nil {
			break
		}
	}
	_ = writer.Close()
}
