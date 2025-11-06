<script lang="ts">
  import DataTable from "../../components/data-table.svelte";
  import { Button } from "$lib/components/ui/button/index";
  import * as Dialog from "$lib/components/ui/dialog/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import type { ColumnDef } from "@tanstack/table-core";

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
      cell: (info) => {
        const item = info.row.original;
        return {
          render: () => null,
          data: item
        };
      },
      enableHiding: false,
    },
  ];

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
  <DataTable 
    data={listings} 
    {columns}
    filterPlaceholder="Filter items..."
    showFilter={true}
    showColumnVisibility={true}
    showPagination={true}
    actions={[
      {
        label: "Add New Item",
        onclick: openAddDialog
      }
    ]}
  >
    {#snippet actionsColumn({ row })}
      <div class="flex justify-end gap-2">
        <Button variant="outline" size="sm" onclick={() => openEditDialog(row.original)}>
          Edit
        </Button>
        <Button variant="destructive" size="sm" onclick={() => deleteItem(row.original.id)}>
          Delete
        </Button>
      </div>
    {/snippet}
  </DataTable>
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