# Chirpy

A modern REST API server inspired by Twitter, built with Go. Chirpy provides a secure and scalable backend for creating and managing short messages ("chirps"), user authentication, and premium features.

This project is a part of the [Boot.dev](https://boot.dev) course.

## Features

- 🔐 Secure user authentication with JWT tokens
- 📝 Create, read, and manage chirps (short messages)
- 👤 User management with email/password
- ⭐ Premium features with Chirpy Red
- 🔄 Refresh token functionality
- 📊 PostgreSQL database for persistent storage

## Prerequisites

- Go 1.24.1 or higher
- PostgreSQL database
- API key for Polka integration (for premium features)

## Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/babanini95/chirpy.git
   cd chirpy
   ```

2. Install dependencies:

   ```bash
   go mod download
   ```

3. Set up your environment variables:

   ```bash
   cp .env.example .env
   # Edit .env with your database credentials and Polka API key
   ```

4. Set up the database:

   ```bash
   source .env
   cd sql/schema
   goose postgres $DB_URL up
   ```

## Running the Server

Start the server with:

```bash
go run .
```

The server will start on the default port.

## API Documentation

The API provides endpoints for:

- User authentication (login/register)
- Chirp creation and retrieval
- User profile management
- Premium feature activation

For detailed API documentation, please refer to the API documentation (TODO).

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
