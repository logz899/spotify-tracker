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

![image](https://github.com/LinChiaWei/spotify_project/assets/62389828/16ce624d-34a9-4629-a7f8-3e072f43e92f)

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

### Future works  
  * Analysis of playlists with model.
  * Recommendation system.
