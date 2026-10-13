ALTER TABLE pending_spotify_tokens
    DROP CONSTRAINT pending_spotify_tokens_account_id_not_blank,
    DROP COLUMN profile_url,
    DROP COLUMN avatar_url,
    DROP COLUMN display_name,
    DROP COLUMN account_id;

ALTER TABLE spotify_accounts
    DROP CONSTRAINT spotify_accounts_account_id_not_blank,
    DROP CONSTRAINT spotify_accounts_account_id_key,
    DROP CONSTRAINT spotify_accounts_spotify_user_id_key,
    DROP CONSTRAINT spotify_accounts_pkey,
    DROP COLUMN profile_url,
    DROP COLUMN avatar_url,
    DROP COLUMN display_name,
    DROP COLUMN account_id,
    ADD CONSTRAINT spotify_accounts_pkey PRIMARY KEY (spotify_user_id),
    ADD CONSTRAINT spotify_accounts_user_id_key UNIQUE (user_id);
