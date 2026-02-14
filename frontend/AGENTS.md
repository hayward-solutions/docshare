# FRONTEND KNOWLEDGE BASE

## OVERVIEW
Next.js 16 frontend for DocShare, providing a responsive web interface for file management, sharing, and group collaboration.

## STRUCTURE
- `src/app/`: App Router routes, organized by group: `(auth)`, `(dashboard)`, `(public)`.
- `src/components/`: Feature components (e.g., `file-viewer.tsx`) and `ui/` (shadcn primitives).
- `src/contexts/`: React Context providers for global state (e.g., activity notifications).
- `src/hooks/`: Custom React hooks for shared logic.
- `src/lib/`: API client (`api.ts`), TypeScript definitions (`types.ts`), and utility functions.

## WHERE TO LOOK
- Route definitions: `src/app/(dashboard)/`
- API interaction: `src/lib/api.ts`
- Shared UI components: `src/components/ui/`
- Global state: `src/contexts/`

## CONVENTIONS
- **API**: Use `apiMethods` from `@/lib/api` for all backend communication.
- **Components**: Prefer Server Components for data fetching where possible; use Client Components (`'use client'`) for interactive dashboard features.
- **Styling**: Use Tailwind CSS classes exclusively; avoid inline styles.
- **Auth**: Authentication tokens are stored in `localStorage` and automatically handled by the `api` wrapper.
- **Icons**: Use `lucide-react` for all iconography.

## ANTI-PATTERNS
- **Direct Fetch**: Do not use `fetch` directly; use the `api` wrapper in `src/lib/api.ts` for consistent auth and error handling.
- **Monoliths**: Avoid large, monolithic components; decompose into smaller pieces in `src/components/`.
- **TypeScript**: Avoid using `any`; leverage types defined in `src/lib/types.ts`.
- **State**: Do not use global state for data that can be managed locally or via URL parameters.
