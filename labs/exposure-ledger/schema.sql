CREATE TABLE IF NOT EXISTS client_exposure(
  client_id    BIGINT PRIMARY KEY,
  outstanding  BIGINT NOT NULL DEFAULT 0,
  limit_amount BIGINT NOT NULL
);
CREATE TABLE IF NOT EXISTS disbursement(
  id         BIGSERIAL PRIMARY KEY,
  client_id  BIGINT NOT NULL,
  amount     BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO client_exposure(client_id, outstanding, limit_amount) VALUES (1, 0, 1000)
  ON CONFLICT (client_id) DO UPDATE SET outstanding = 0, limit_amount = 1000;
