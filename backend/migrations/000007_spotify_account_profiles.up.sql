ALTER TABLE spotify_accounts
    DROP CONSTRAINT spotify_accounts_pkey,
    DROP CONSTRAINT spotify_accounts_user_id_key,
    ADD COLUMN account_id TEXT,
    ADD COLUMN display_name TEXT,
    ADD COLUMN avatar_url TEXT,
    ADD COLUMN profile_url TEXT,
    ADD CONSTRAINT spotify_accounts_pkey PRIMARY KEY (user_id),
    ADD CONSTRAINT spotify_accounts_spotify_user_id_key UNIQUE (spotify_user_id),
    ADD CONSTRAINT spotify_accounts_account_id_key UNIQUE (account_id),
    ADD CONSTRAINT spotify_accounts_account_id_not_blank
        CHECK (account_id IS NULL OR btrim(account_id) <> '');

ALTER TABLE pending_spotify_tokens
    ADD COLUMN account_id TEXT,
    ADD COLUMN display_name TEXT,
    ADD COLUMN avatar_url TEXT,
    ADD COLUMN profile_url TEXT;

DELETE FROM pending_spotify_tokens;

ALTER TABLE pending_spotify_tokens
    ALTER COLUMN account_id SET NOT NULL,
    ADD CONSTRAINT pending_spotify_tokens_account_id_not_blank
        CHECK (btrim(account_id) <> '');
