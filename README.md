# Spotify Song Rank Web

### Do you know Spotify Wrapped?
  Spotify Wrapped is a feature that Spotify offers to users every year. It shows the user's top artists, songs, genres and podcasts that litened to on Spotify throughout the year. User can also see how many minutes spending listening to Spotify and get personalized playlists based on the performance.

### Motivation
  However, you have to wait for a year to get the  Wrapped. To me, I will listen to different types of songs through different seasons. The analysis of the user performance would be messy. It will be nice if there is a website can show the top rank of my listening habbit every month or season!


### Current progress
#### Backend server  
  * Get user's information from Apis.  
  * Store data into database.  
  * Get data from database.  
  * Fetch data automatically.
#### Frontend
  * Connect the server and get data.
  * Display the data on the website.
  * Data display with different time intervals.
  * Use one accessible profile menu for Spotify connection state, manual sync,
    Spotify profile access, and logout.

<img width="2988" height="1723" alt="image" src="https://github.com/user-attachments/assets/d159dabf-b3be-4891-acf7-8f53af0bd225" />


### Architecture and hosting
  * **Frontend:** React, published as a static site on GitHub Pages.
  * **API:** Go/Gin on Google Cloud Run (scale 0–1, billed only while serving requests).
  * **Database:** Neon Free PostgreSQL over TLS.
  * **Sync:** a private Cloud Run Job started every 15 minutes by one Cloud Scheduler job.
  * **Secrets:** four values in Google Secret Manager (`spotify-client-secret`, `jwt-secret`, `database-url`, `token-encryption-key`).

  The setup is designed to stay within free tiers for hobby traffic. Artifact Registry
  keeps only the 3 newest images, and a US$1 budget alerts at 50/90/100%. **Budget alerts
  are notifications, not a spending cap**; Google Cloud does not stop services when they fire.
  See [`deploy/cloudrun/README.md`](deploy/cloudrun/README.md) for setup and release steps.

### Accounts and Spotify profiles

The application account is primary. Spotify is an optional linked identity and
data source: users can sign in with application email/password, then connect
Spotify, or start with Spotify and finish creating an application account.

Spotify links use the private, immutable `account_id` returned by the current
user profile endpoint. The legacy Spotify `id` is retained only as migration
metadata. Neither identifier, Spotify tokens, nor application email is returned
by the profile API. The UI receives only the application username, connection
state, sync status, and public Spotify display name/avatar/profile URL.

The profile menu uses the Spotify avatar when available and otherwise falls
back to the first character of the application username. It supports pointer
and keyboard use, closes on outside interaction or Escape, and hides Connect
and Sync when profile status is temporarily unavailable.

This release adds no OAuth scope and never requests Spotify email. Existing
development scopes remain unchanged. Spotify Development Mode currently limits
new apps to five authorized users and requires the app owner to maintain a
Premium account.

Production database migration and deployment are separate, approval-gated
operations; running the project locally does not change Neon, Cloud Run,
GitHub Pages, or Spotify Dashboard settings.

### Future works  
  * Analysis of playlists with model.
  * Recommendation system.
