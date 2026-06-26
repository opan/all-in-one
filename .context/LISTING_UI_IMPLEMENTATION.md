# Listing UI Implementation

## Overview

Created a complete listing page UI with navigation menu, collapsible sidebar using shadcn-svelte Sidebar component, and data table based on the provided design mockup.

## Components Added

### 1. Theme Toggle Component

**File**: `src/lib/components/theme-toggle.svelte`

- Dropdown menu for switching between light and dark themes
- Persists theme preference in localStorage
- Sun icon for light mode, moon icon for dark mode

### 2. App Sidebar Component

**File**: `src/lib/components/app-sidebar.svelte`

- Reusable sidebar using shadcn-svelte Sidebar components
- Navigation menu with icons (House, LayoutDashboard, Settings)
- Active state highlighting based on current route
- Includes sidebar rail for easy toggle

### 3. shadcn-svelte Components Installed

- ✅ Sidebar component (complete sidebar system)
- ✅ Card component (for the main content container)
- ✅ Dropdown Menu component (for theme toggle)
- ✅ Table, Button, Dialog, Input, Label (already existed)

## Main Features

### Layout Structure

1. **Sidebar.Provider**
   - Wraps the entire page for sidebar state management
   - Handles sidebar open/close state
   - Keyboard shortcut support (Cmd/Ctrl + B)

2. **AppSidebar Component**
   - Sticky sidebar with "SIDEBAR MENU" header
   - Navigation menu items:
     - Listings (with House icon)
     - Dashboard (with LayoutDashboard icon)
     - Settings (with Settings icon)
   - Collapsible with smooth animations
   - Rail component for easy toggle
   - Responsive to screen size

3. **Top Navigation Bar (in Sidebar.Inset)**
   - Sidebar.Trigger button (hamburger menu)
   - "NAVIGATION MENU" title
   - Theme toggle button on the right

4. **Main Content Area**
   - Card container with "TABLE" header
   - "Add New Item" button in card header
   - Data table with columns:
     - ID
     - Title
     - Description
     - Created date
     - Updated date
     - Actions (Edit/Delete buttons)

### Functionality

- ✅ Add new items via dialog
- ✅ Edit existing items
- ✅ Delete items (with confirmation)
- ✅ Toggle sidebar visibility (via trigger or keyboard shortcut)
- ✅ Switch between light/dark themes
- ✅ Dummy data pre-populated for demonstration
- ✅ Icon-based navigation with active states

### Styling

- Uses Tailwind CSS throughout
- Follows shadcn-svelte design system
- Fully responsive layout
- Dark mode support with proper color tokens (including sidebar-specific tokens)
- Smooth transitions and hover effects
- Professional sidebar with proper spacing and typography

## Sidebar Features

The shadcn-svelte Sidebar component provides:

- **Collapsible states**: offcanvas, icon, or none
- **Variants**: sidebar, floating, or inset (currently using default)
- **Keyboard shortcuts**: Cmd/Ctrl + B to toggle
- **Mobile responsive**: Automatically adapts to mobile screens
- **Rail component**: Visual affordance for toggling
- **Active state management**: Highlights current route
- **Customizable width**: Via CSS variables

## Theme Support

The implementation includes full dark/light theme support:

- Theme toggle in navigation bar
- Persists preference in localStorage
- Automatic theme application on page load
- Uses CSS custom properties for seamless switching
- Sidebar-specific color tokens for independent theming

## Dummy Data

The page includes 4 sample items for demonstration:

1. First Item
2. Second Item
3. Third Item
4. Fourth Item

Each with descriptions and timestamps.

## JSONForms Integration

Topics now support custom form schemas using [JSONForms.io](https://jsonforms.io/) format. This allows flexible definition of custom fields for items within each topic.

### Schema Format

Each topic can define:

- **JSON Schema** - Data structure and validation rules (standard JSON Schema format)
- **UI Schema** - Layout and rendering instructions (JSONForms UI Schema format)

See [JSONFORMS_SCHEMA_GUIDE.md](./JSONFORMS_SCHEMA_GUIDE.md) for detailed documentation and examples.

### Features

- Real-time JSON validation
- Visual preview of schema properties
- Support for all standard JSON Schema types and formats
- Flexible UI layouts (Vertical, Horizontal, Groups)
- Link to official JSONForms documentation

## Next Steps

To connect to the real API:

1. Uncomment the API integration code in `+page.svelte`
2. Update the data loading in `+page.ts` to fetch from `/api/v1/items`
3. The API endpoints are already integrated:
   - GET `/api/v1/items` - List all items
   - POST `/api/v1/items` - Create new item
   - PUT `/api/v1/items/:id` - Update item
   - DELETE `/api/v1/items/:id` - Delete item

## File Changes

### Modified Files:

- `web/src/routes/listing/+page.svelte` - Complete UI implementation with Sidebar.Provider
- `web/src/routes/+layout.svelte` - Added background container

### New Files:

- `web/src/lib/components/theme-toggle.svelte` - Theme switcher component
- `web/src/lib/components/app-sidebar.svelte` - Reusable sidebar component
- `web/src/lib/components/ui/card/*` - Card components
- `web/src/lib/components/ui/dropdown-menu/*` - Dropdown menu components
- `web/src/lib/components/ui/sidebar/*` - Complete sidebar component system

## Technical Details

### Sidebar Component Structure

The implementation uses the official shadcn-svelte sidebar pattern:

```svelte
<Sidebar.Provider>
  <AppSidebar />
  <Sidebar.Inset>
    <!-- Navigation and content here -->
  </Sidebar.Inset>
</Sidebar.Provider>
```

### Icons

Uses `@lucide/svelte/icons` for consistent iconography:

- House icon for Listings
- LayoutDashboard icon for Dashboard
- Settings icon for Settings

### Active State Detection

Currently uses `window.location.pathname` for active state detection. Can be enhanced with SvelteKit's `$page` store for more robust routing.
