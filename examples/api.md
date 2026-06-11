# API Routes

Generated: 2026-01-07 00:47:22

## /api/items

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `GET` | `/api/items/` | Public | items.(*Routes).list |
| `POST` | `/api/items/` | Public | items.(*Routes).create |
| `GET` | `/api/items/{id}` | Public | items.(*Routes).getByID |
| `PUT` | `/api/items/{id}` | Public | items.(*Routes).update |
| `DELETE` | `/api/items/{id}` | Public | items.(*Routes).delete |

## /auth

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `POST` | `/auth/logout` | Required | auth.(*Routes).logout |
| `POST` | `/auth/logout-all` | Required | auth.(*Routes).logoutAll |
| `GET` | `/auth/whoami` | Required | auth.(*Routes).whoami |

## /docs

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `GET` | `/docs` | Public | routes.NewRouter.DocsIndexHandler.<closure> |

## /docs/queries

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `GET` | `/docs/queries/` | Public | routes.NewRouter.QueriesDocHandler.<closure> |
| `GET` | `/docs/queries/json` | Public | routes.NewRouter.QueriesDocHandler.<closure> |

## /docs/routes

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `GET` | `/docs/routes/` | Public | routes.NewRouter.RoutesDocHandler.<closure> |
| `GET` | `/docs/routes/json` | Public | routes.NewRouter.RoutesDocHandler.<closure> |

## /docs/tests

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `GET` | `/docs/tests` | Public | routes.NewRouter.<closure> |
| `GET` | `/docs/tests/*` | Public | routes.NewRouter.<closure> |

## /health

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `GET` | `/health` | Public | routes.NewRouter.<closure> |

## /schema

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `GET` | `/schema/` | Public | routes.NewRouter.SchemaRoutes.<closure> |
| `GET` | `/schema/{name}` | Public | routes.NewRouter.SchemaRoutes.<closure> |

## Root

| Method | Path | Auth | Handler |
|--------|------|------|--------|
| `GET` | `/` | Public | routes.NewRouter.<closure> |

## Summary

- **Total routes:** 19
- **Authenticated:** 3
- **Public:** 16
