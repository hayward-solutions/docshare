# UI COMPONENTS KNOWLEDGE BASE

## OVERVIEW
Reusable UI primitives built on Radix UI and styled with Tailwind CSS 4 (shadcn/ui).

## WHERE TO LOOK
| Component | Purpose |
|-----------|---------|
| `button.tsx` | Primary action component with `cva` variants. |
| `dialog.tsx` / `sheet.tsx` | Modal and side-panel overlays for complex interactions. |
| `dropdown-menu.tsx` / `select.tsx` | Form and navigation selection components. |
| `table.tsx` | Data grid layout components for file listings. |
| `sonner.tsx` | Toast notification system for async feedback. |
| `scroll-area.tsx` | Custom scrollbar implementation for long lists. |

## CONVENTIONS
- **Radix UI**: Most components are thin wrappers around Radix primitives for accessibility.
- **Styling**: Strictly Tailwind CSS 4 classes via the `cn()` utility.
- **Variants**: Defined using `class-variance-authority` (CVA) for consistent state styling.
- **Refs**: All components use `React.forwardRef` to ensure compatibility with Radix and animation libraries.
- **Composition**: Prefer small, composable sub-components (e.g., `DialogHeader`, `DialogContent`).

## ANTI-PATTERNS
- **No Business Logic**: These are pure presentational primitives. Never import services or hooks with side effects.
- **No Hardcoded Colors**: Use theme variables (e.g., `bg-background`, `text-primary`) to support future theming.
- **Direct Modification**: Avoid changing these files for one-off styling; use `className` props or create a wrapper component instead.
- **Prop Drilling**: Avoid deep prop drilling; use the provided sub-components for composition.
