CREATE TABLE
  refresh_token (
    user_id UUID NOT NULL,
    device_id BYTEA NOT NULL,
    token_hash BYTEA NOT NULL,
    PRIMARY KEY (user_id, device_id)
  );
