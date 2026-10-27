# PaddleOCR não publica uma imagem de serviço pronta: o projeto expõe o
# pipeline por HTTP através do PaddleX, que precisa ser instalado junto.
# A porta 8080 e a rota POST /ocr seguem o padrão do `paddlex --serve`.
FROM python:3.11-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends curl libgl1 libglib2.0-0 libgomp1 \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
      paddlepaddle==3.0.0 \
      "paddleocr[all]==3.2.0" \
      "paddlex[serving]==3.2.0"

EXPOSE 8080
CMD ["paddlex", "--serve", "--pipeline", "OCR", "--host", "0.0.0.0", "--port", "8080"]
