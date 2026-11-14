-- M1.3: whether a connection's database password is in the keychain, so the
-- sidebar can show it without a keychain read per connection. Only the state
-- is stored here, never the password:
--   none      no password is saved, and none is expected
--   saved     the keychain holds the password
--   required  a password is needed but not saved, e.g. after an import (PLAN §3.6)

ALTER TABLE connections ADD COLUMN password_state TEXT NOT NULL DEFAULT 'none';
