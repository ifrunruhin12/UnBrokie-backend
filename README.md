# Money Manager for Noobs

A minimal personal finance backend focused on balance tracking, financial projections, and learning backend engineering through building real systems.

Built with Go and PostgreSQL.

---

## Why this exists

I originally built this project to fix my own terrible money management habits while learning backend engineering properly.

Instead of using a generic finance app, I wanted something:

* personalized
* minimal
* simulation-focused
* and architected in a way that teaches me real backend concepts

This project became a playground for learning:

* Go (Golang)
* Backend architecture
* PostgreSQL
* Authentication systems
* Financial data consistency
* Transaction-safe operations

---

## Features

### Authentication

* User registration & login
* JWT-based authentication
* Password hashing with bcrypt
* Protected API routes

### Finance System

* Account balance tracking
* Transaction management
* Skip/restore transactions
* Override transaction system
* Big buy tracking
* Category-based expenses
* Event logging

### Architecture

* Clean layered design
* Repository + service pattern
* PostgreSQL with migrations
* Transaction-safe database operations
* Atomic balance updates
* Dockerized development setup

---

## Tech Stack

* Go (Golang)
* PostgreSQL
* Gin
* JWT
* Docker
* pgx

---

## Current Status

* Core backend architecture mostly complete
* Authentication system implemented
* Financial transaction system implemented
* Big buy system implemented
* API handlers/routes in progress
* Flutter frontend planned next

---

## Goal

Build a personalized finance system that helps me:

* understand spending habits
* simulate financial decisions
* track money properly
* and grow into a better backend engineer while building real software

---

## Notes

This is currently an MVP and learning-focused project.

The system is intentionally designed to evolve over time as I improve the architecture, add features, and scale the project beyond personal use.