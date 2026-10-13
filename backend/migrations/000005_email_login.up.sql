UPDATE users
SET email = lower(btrim(email));

CREATE UNIQUE INDEX users_email_lower_unique
    ON users (lower(email));
