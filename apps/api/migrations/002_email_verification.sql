CREATE TABLE IF NOT EXISTS email_verifications (token_hash CHAR(64) PRIMARY KEY, user_id CHAR(32) NOT NULL, payload JSON NOT NULL, FOREIGN KEY(user_id) REFERENCES users(id))
