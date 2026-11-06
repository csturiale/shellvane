-- Recovery key: the data key also wrapped by a key derived from a random
-- recovery code, shown once to the user.
ALTER TABLE vault_meta ADD COLUMN recovery_salt BLOB;
ALTER TABLE vault_meta ADD COLUMN recovery_time INTEGER;
ALTER TABLE vault_meta ADD COLUMN recovery_memory INTEGER;
ALTER TABLE vault_meta ADD COLUMN recovery_threads INTEGER;
ALTER TABLE vault_meta ADD COLUMN recovery_wrapped BLOB;
