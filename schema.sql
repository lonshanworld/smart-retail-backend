-- This file contains the SQL commands to set up your database schema.
-- You can add CREATE TABLE, ALTER TABLE, and INSERT statements here.
-- This file can be used to set up your production database.

-- Example: A simple 'users' table.
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Example: Insert a sample user.
-- INSERT INTO users (name, email) VALUES ('John Doe', 'john.doe@example.com');
