<script lang="ts">
  import { createSvelteTable } from "$lib/components/ui/data-table/data-table.svelte";
  import { FlexRender } from "$lib/components/ui/data-table";
  import * as Table from "$lib/components/ui/table/index";
  import { Button } from "$lib/components/ui/button/index";
  import * as Dialog from "$lib/components/ui/dialog/index";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import { 
    getCoreRowModel, 
    getFilteredRowModel,
    getPaginationRowModel,
    type ColumnDef 
  } from "@tanstack/table-core";
  import { ChevronDown } from "@lucide/svelte";

  interface Item {
    id: number;
    title: string;
    description: string;
    created_at: string;
    updated_at: string;
  }

  let listings = $state<Item[]>([
    { id: 1, title: "First Item", description: "Description for first item", created_at: "2025-10-15T10:00:00Z", updated_at: "2025-10-15T10:00:00Z" },
    { id: 2, title: "Second Item", description: "Description for second item", created_at: "2025-10-16T11:30:00Z", updated_at: "2025-10-16T11:30:00Z" },
    { id: 3, title: "Third Item", description: "Description for third item", created_at: "2025-10-17T09:15:00Z", updated_at: "2025-10-17T09:15:00Z" },
    { id: 4, title: "Fourth Item", description: "Description for fourth item", created_at: "2025-10-18T14:20:00Z", updated_at: "2025-10-18T14:20:00Z" },
  ]);
  
  let dialogOpen = $state(false);
  let editingItem = $state<number | null>(null);
  let formData = $state({
    title: '',
    description: ''
  });
  
  let loading = $state(false);
  let error = $state('');
  let filtering = $state('');
  let columnVisibility = $state<Record<string, boolean>>({});

  const columns: ColumnDef<Item>[] = [
    {
      accessorKey: "id",
      header: "ID",
      cell: (info) => info.getValue(),
      enableHiding: true,
    },
    {
      accessorKey: "title",
      header: "Title",
      cell: (info) => info.getValue(),
      enableHiding: true,
    },
    {
      accessorKey: "description",
      header: "Description",
      cell: (info) => info.getValue(),
      enableHiding: true,
    },
    {
      accessorKey: "created_at",
      header: "Created",
      cell: (info) => formatDate(info.getValue() as string),
      enableHiding: true,
    },
    {
      accessorKey: "updated_at",
      header: "Updated",
      cell: (info) => formatDate(info.getValue() as string),
      enableHiding: true,
    },
    {
      id: "actions",
      header: "Actions",
      cell: () => null,
      enableHiding: false,
    },
  ];

  const table = createSvelteTable({
    get data() {
      return listings;
    },
    columns,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    onColumnVisibilityChange: (updater) => {
      if (typeof updater === 'function') {
        columnVisibility = updater(columnVisibility);
      } else {
        columnVisibility = updater;
      }
    },
    onGlobalFilterChange: (updater) => {
      if (typeof updater === 'function') {
        filtering = updater(filtering);
      } else {
        filtering = updater;
      }
    },
    state: {
      get globalFilter() {
        return filtering;
      },
      get columnVisibility() {
        return columnVisibility;
      },
    },
  });

  function openAddDialog() {
    editingItem = null;
    formData = { title: '', description: '' };
    dialogOpen = true;
  }

  function openEditDialog(item: Item) {
    editingItem = item.id;
    formData = { title: item.title, description: item.description };
    dialogOpen = true;
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();

    if (!formData.title.trim() || !formData.description.trim()) {
      error = 'Title and description are required';
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      if (editingItem) {
        const response = await fetch(`/api/v1/items/${editingItem}`, {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            title: formData.title.trim(),
            description: formData.description.trim()
          }),
        });
        
        if (!response.ok) {
          throw new Error('Failed to update item');
        }
        
        const updatedItem = await response.json();
        const itemIndex = listings.findIndex(item => item.id === editingItem);
        if (itemIndex !== -1) {
          listings[itemIndex] = updatedItem.data;
          listings = listings;
        }
      } else {
        const response = await fetch('/api/v1/items', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            title: formData.title.trim(),
            description: formData.description.trim()
          }),
        });
        
        if (!response.ok) {
          throw new Error('Failed to create item');
        }
        
        const newItem = await response.json();
        listings.push(newItem.data);
        listings = listings;
      }
      
      formData = { title: '', description: '' };
      editingItem = null;
      dialogOpen = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }
  
  async function deleteItem(id: number) {
    if (!confirm('Are you sure you want to delete this item?')) {
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      const response = await fetch(`/api/v1/items/${id}`, {
        method: 'DELETE',
      });
      
      if (!response.ok) {
        throw new Error('Failed to delete item');
      }
      
      const itemIndex = listings.findIndex(item => item.id === id);
      if (itemIndex !== -1) {
        listings.splice(itemIndex, 1);
        listings = listings;
      }
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }
  
  function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
  }
</script>

<div class="container mx-auto p-6">
  <div class="flex items-center justify-between gap-4 mb-4">
    <Input
      placeholder="Filter items..."
      value={filtering}
      oninput={(e) => filtering = e.currentTarget.value}
      class="max-w-sm"
    />
    <div class="flex items-center gap-2">
      <DropdownMenu.Root>
        <DropdownMenu.Trigger>
          <Button variant="outline" class="ml-auto">
            Columns <ChevronDown class="ml-2 h-4 w-4" />
          </Button>
        </DropdownMenu.Trigger>
        <DropdownMenu.Content align="end">
          {#each table.getAllLeafColumns() as column}
            {#if column.getCanHide()}
              <DropdownMenu.CheckboxItem
                checked={column.getIsVisible()}
                onCheckedChange={(value) => column.toggleVisibility(!!value)}
              >
                {column.id}
              </DropdownMenu.CheckboxItem>
            {/if}
          {/each}
        </DropdownMenu.Content>
      </DropdownMenu.Root>
      <Button onclick={openAddDialog}>Add New Item</Button>
    </div>
  </div>

  <div class="rounded-md border">
    <Table.Root>
      <Table.Header>
        {#each table.getHeaderGroups() as headerGroup}
          <Table.Row>
            {#each headerGroup.headers as header}
              <Table.Head>
                {#if !header.isPlaceholder}
                  <FlexRender
                    content={header.column.columnDef.header}
                    context={header.getContext()}
                  />
                {/if}
              </Table.Head>
            {/each}
          </Table.Row>
        {/each}
      </Table.Header>
      <Table.Body>
        {#if table.getRowModel().rows?.length}
          {#each table.getRowModel().rows as row}
            <Table.Row>
              {#each row.getVisibleCells() as cell}
                <Table.Cell>
                  {#if cell.column.id === 'actions'}
                    <div class="flex justify-end gap-2">
                      <Button variant="outline" size="sm" onclick={() => openEditDialog(row.original)}>
                        Edit
                      </Button>
                      <Button variant="destructive" size="sm" onclick={() => deleteItem(row.original.id)}>
                        Delete
                      </Button>
                    </div>
                  {:else}
                    <FlexRender
                      content={cell.column.columnDef.cell}
                      context={cell.getContext()}
                    />
                  {/if}
                </Table.Cell>
              {/each}
            </Table.Row>
          {/each}
        {:else}
          <Table.Row>
            <Table.Cell colspan={columns.length} class="h-24 text-center">
              No results.
            </Table.Cell>
          </Table.Row>
        {/if}
      </Table.Body>
    </Table.Root>
  </div>

  <div class="flex items-center justify-between space-x-2 py-4">
    <div class="text-sm text-muted-foreground">
      {table.getFilteredRowModel().rows.length} row(s) total.
    </div>
    <div class="flex items-center space-x-2">
      <Button
        variant="outline"
        size="sm"
        onclick={() => table.previousPage()}
        disabled={!table.getCanPreviousPage()}
      >
        Previous
      </Button>
      <Button
        variant="outline"
        size="sm"
        onclick={() => table.nextPage()}
        disabled={!table.getCanNextPage()}
      >
        Next
      </Button>
    </div>
  </div>
</div>

<Dialog.Root bind:open={dialogOpen}>
  <Dialog.Content class="sm:max-w-[425px]">
    <Dialog.Header>
      <Dialog.Title>{editingItem ? 'Edit Item' : 'Add New Item'}</Dialog.Title>
      <Dialog.Description>
        {editingItem ? 'Make changes to your item here.' : 'Add a new item to your listing.'}
      </Dialog.Description>
    </Dialog.Header>

    <form onsubmit={handleSubmit} class="space-y-4">
      {#if error}
        <div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">
          {error}
        </div>
      {/if}

      <div class="space-y-2">
        <Label for="title">Title</Label>
        <Input id="title" type="text" bind:value={formData.title} required />
      </div>

      <div class="space-y-2">
        <Label for="description">Description</Label>
        <Input id="description" type="text" bind:value={formData.description} required />
      </div>

      <Dialog.Footer>
        <Button type="button" variant="outline" onclick={() => { dialogOpen = false; editingItem = null; }} disabled={loading}>
          Cancel
        </Button>
        <Button type="submit" disabled={loading}>
          {loading ? 'Saving...' : editingItem ? 'Save changes' : 'Add item'}
        </Button>
      </Dialog.Footer>
    </form>
  </Dialog.Content>
</Dialog.Root>