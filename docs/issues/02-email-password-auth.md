# 02 — Email & Password Authentication (Login, Session, Basic Roles)

## What to build

Wire up email & password authentication for user login. A user who logs in with their email and password gets a session. On first login, the system creates a user record and assigns a default role. Super-admins can be bootstrapped via an environment variable allowlist.

Deliver:
- Registration endpoint that accepts email & password, hashes password with bcrypt, and stores the user
- Login endpoint that validates email & password against the stored hash and issues a session cookie
- User record created on registration with email, display name, and role
- Roles: `super_admin`, `admin`, `viewer` — `admin` is the default for new registrations
- Super-admin bootstrap via `SUPER_ADMIN_EMAILS` environment variable
- Protected route middleware on the Go backend (rejects unauthenticated requests to `/api/admin/*`)
- SvelteKit login page (email & password form) and a minimal authenticated admin shell (nav + "logged in as X" header)
- Logout endpoint that clears the session

## Acceptance criteria

- [ ] `POST /api/auth/register` accepts email and password, hashes password with bcrypt, and creates a user with `admin` role
- [ ] Emails listed in `SUPER_ADMIN_EMAILS` receive `super_admin` role on registration
- [ ] `POST /api/auth/login` validates email & password against stored hash and returns a valid session cookie on success
- [ ] `POST /api/auth/login` returns `401` for invalid credentials
- [ ] `GET /api/admin/me` returns the authenticated user's profile and role
- [ ] `GET /api/admin/me` returns `401` with no session
- [ ] Login form on `/login` accepts email and password input and submits to `/api/auth/login`
- [ ] After successful login, user is redirected to `/admin` with a valid session cookie
- [ ] Logout clears the session and redirects to `/login`
- [ ] The admin shell renders the logged-in user's name and role

## Blocked by

- #01 Project Scaffold
