# 🚀 Deployment Guide - PostgreSQL Database Setup

This guide explains how to set up PostgreSQL database for production hosting.

## 📋 Prerequisites

- PostgreSQL database (hosted on services like Render, Railway, Supabase, or AWS RDS)
- Database connection string (DATABASE_URL)

## 🔧 Setting Up Database for Production

### Option 1: Using Render.com (Recommended)

1. **Create a PostgreSQL Database on Render:**
   - Go to [Render Dashboard](https://dashboard.render.com)
   - Click "New +" → "PostgreSQL"
   - Fill in:
     - **Name**: `connect4-db` (or your preferred name)
     - **Database**: `connect_four`
     - **User**: `connect4_user` (or auto-generated)
     - **Region**: Choose closest to your backend
   - Click "Create Database"

2. **Get Connection String:**
   - After creation, go to your database dashboard
   - Copy the "Internal Database URL" or "External Database URL"
   - Format: `postgres://user:password@host:port/database?sslmode=require`

3. **Set Environment Variable:**
   - In your backend service on Render:
     - Go to "Environment" tab
     - Add environment variable:
       - **Key**: `DATABASE_URL`
       - **Value**: Paste your connection string

### Option 2: Using Railway.app

1. **Create PostgreSQL Service:**
   - Go to [Railway Dashboard](https://railway.app)
   - Click "New Project" → "Database" → "PostgreSQL"
   - Railway automatically creates the database

2. **Get Connection String:**
   - Click on your PostgreSQL service
   - Go to "Variables" tab
   - Copy the `DATABASE_URL` value

3. **Connect to Backend:**
   - In your backend service:
     - Go to "Variables" tab
     - Add `DATABASE_URL` variable
     - Railway automatically provides it if services are in same project

### Option 3: Using Supabase

1. **Create Project:**
   - Go to [Supabase Dashboard](https://supabase.com)
   - Create a new project
   - Wait for database to be provisioned

2. **Get Connection String:**
   - Go to Project Settings → Database
   - Copy "Connection string" → "URI"
   - Format: `postgres://postgres:[YOUR-PASSWORD]@db.[PROJECT-REF].supabase.co:5432/postgres`

3. **Set Environment Variable:**
   - In your backend hosting service:
     - Add `DATABASE_URL` environment variable with the connection string

### Option 4: Using AWS RDS

1. **Create RDS PostgreSQL Instance:**
   - Go to AWS Console → RDS
   - Create PostgreSQL database
   - Note: Enable public access if needed

2. **Get Connection String:**
   - Format: `postgres://username:password@hostname:5432/connect_four?sslmode=require`

3. **Set Environment Variable:**
   - Add `DATABASE_URL` to your backend environment

## 🔐 Environment Variables for Backend

Add these environment variables to your backend hosting service:

```bash
# Required for database connection
DATABASE_URL=postgres://user:password@host:port/database?sslmode=require

# Optional - CORS origins (comma-separated)
CORS_ORIGINS=https://your-frontend-domain.com,https://www.your-frontend-domain.com

# Optional - Kafka brokers (if using Kafka)
KAFKA_BROKERS=your-kafka-broker:9092

# Optional - Bot difficulty
BOT_DIFFICULTY=normal
```

## ✅ Verification

After setting up the database:

1. **Check Backend Logs:**
   - Look for: `Successfully connected to PostgreSQL database`
   - If you see errors, check the connection string format

2. **Test Health Endpoint:**
   ```bash
   curl https://your-backend-url.com/health
   ```
   - Should show: `"database": true`

3. **Test Leaderboard:**
   ```bash
   curl https://your-backend-url.com/leaderboard
   ```
   - Should return empty array `[]` if no games played yet
   - Should NOT return mock data like "Player1", "Player2"

4. **Play a Game:**
   - Play a complete game
   - Check leaderboard again - should show real usernames

## 🐛 Troubleshooting

### Issue: Database connection fails

**Symptoms:**
- Backend logs show: `ERROR: Could not connect to database`
- Leaderboard shows empty array (not mock data)

**Solutions:**
1. **Check DATABASE_URL format:**
   - Must be: `postgres://user:password@host:port/database?sslmode=require`
   - Special characters in password must be URL-encoded

2. **Check SSL Mode:**
   - For production: Use `sslmode=require`
   - For local: Use `sslmode=disable`

3. **Check Firewall:**
   - Ensure your database allows connections from your backend IP
   - Some services require IP whitelisting

4. **Check Database Status:**
   - Verify database is running and accessible
   - Check database credentials

### Issue: Leaderboard shows "Player1", "Player2"

**Cause:** Database connection failed, so backend returned mock data

**Solution:**
- This is now fixed - backend returns empty array instead of mock data
- Ensure DATABASE_URL is set correctly
- Check backend logs for database connection errors

### Issue: Tables don't exist

**Solution:**
- The backend now automatically creates tables on startup
- Check logs for: `Database migrations completed successfully`
- If migration fails, check database permissions

## 📊 Database Schema

The backend automatically creates these tables:

```sql
CREATE TABLE games (
    id VARCHAR(50) PRIMARY KEY,
    player1_id VARCHAR(100) NOT NULL,
    player1_username VARCHAR(50) NOT NULL,
    player2_id VARCHAR(100) NOT NULL,
    player2_username VARCHAR(50) NOT NULL,
    winner_id VARCHAR(100),
    move_count INTEGER NOT NULL DEFAULT 0,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMP,
    is_bot_game BOOLEAN NOT NULL DEFAULT FALSE
);
```

## 🔄 Automatic Migrations

The backend automatically runs migrations on startup:
- Creates `games` table if it doesn't exist
- Creates indexes for performance
- No manual migration needed

## 📝 Notes

- Database connection is optional - backend works without it (but won't save games)
- Games are saved automatically when they complete
- Leaderboard updates in real-time after games finish
- Database connection includes retry logic (5 attempts with exponential backoff)
