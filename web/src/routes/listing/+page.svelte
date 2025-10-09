<script lang="ts">
  import * as Table from "$lib/components/ui/table/index"
  import { Button } from "$lib/components/ui/button/index"
  import * as Dialog from "$lib/components/ui/dialog/index"
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";

  // export let data: { listings: Item[] };
  const { data } = $props<{ data: { listings: Item[] } }>();
  let listings = $state(data.listings);
  
  interface Item {
    id: number;
    title: string;
    description: string;
    created_at: string;
    updated_at: string;
  }
  
  // Form state
  let editingItem = $state<number | null>(null);
  let formData = $state({
    title: '',
    description: ''
  });
  
  // Loading and error states
  let loading = $state(false);
  let error = $state('');
  
  // Add new item
  async function addItem(e: any) {
    e.preventDefault();

    if (!formData.title.trim() || !formData.description.trim()) {
      error = 'Title and description are required';
      return;
    }
    
    loading = true;
    error = '';
    
    try {
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
      console.log('New item created:', newItem);
      listings.push(newItem.data);
      listings = listings; // Force reactivity
      
      // Reset form
      formData = { title: '', description: '' };
      dialogOpen = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }
  
  // Edit item
  function startEdit(item: Item) {
    editingItem = item.id;
    formData = {
      title: item.title,
      description: item.description
    };
    dialogOpen = true;
  }
  
  function cancelEdit() {
    editingItem = null;
    formData = { title: '', description: '' };
    dialogOpen = false;
  }

  async function saveEdit(e: any, id: number) {
    e.preventDefault();

    if (!formData.title.trim() || !formData.description.trim()) {
      error = 'Title and description are required';
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      const response = await fetch(`/api/v1/items/${id}`, {
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
      console.log('Item updated:', updatedItem);
      const itemIndex = listings.findIndex(item => item.id === id);
      if (itemIndex !== -1) {
        listings[itemIndex] = updatedItem.data;
        listings = listings; // Force reactivity
      }
      
      editingItem = null;
      formData = { title: '', description: '' };
      dialogOpen = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }
  
  // Delete item
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
      
      console.log('Item deleted:', id);
      const itemIndex = listings.findIndex(item => item.id === id);
      if (itemIndex !== -1) {
        listings.splice(itemIndex, 1);
        listings = listings; // Force reactivity
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

  // Refresh data from server
  async function refreshListings() {
    try {
      const response = await fetch('/api/v1/items');
      if (response.ok) {
        const result = await response.json();
        listings = result.data || [];
        console.log('Listings refreshed:', listings);
      }
    } catch (err) {
      console.error('Failed to refresh listings:', err);
    }
  }


  let dialogOpen = $state(false);
</script>

<Dialog.Root bind:open={dialogOpen}>
  <Dialog.Portal> 
    <Dialog.Overlay />

    <Dialog.Content preventScroll={false}> 
      <Dialog.Header> 
        <Dialog.Title>{editingItem ? 'Edit Item' : 'Add New Item'}</Dialog.Title>

        <Dialog.Description>{editingItem ? 'Edit the existing item' : 'Create a new item'}</Dialog.Description>

      </Dialog.Header>

      <form onsubmit={(e) => editingItem ? saveEdit(e, editingItem) : addItem(e)} class="space-y-4">
        {#if error}
          <p class="text-red-500">{error}</p>
        {/if}

        <div>
          <Label for="title">Title</Label>
          <Input id="title" type="text" bind:value={formData.title} required />
        </div>

        <div>
          <Label for="description">Description</Label>
          <Input id="description" type="text" bind:value={formData.description} required />
        </div>

        <div class="flex justify-end space-x-2">
          {#if editingItem}
            <Button type="button" variant="secondary" onclick={cancelEdit} disabled={loading}>Cancel</Button>
            <Button type="submit" variant="default" disabled={loading}>{loading ? 'Saving...' : 'Save'}</Button>
          {:else}
            <Button type="button" variant="secondary" onclick={() => dialogOpen = false} disabled={loading}>Cancel</Button>
            <Button type="submit" variant="default" disabled={loading}>{loading ? 'Adding...' : 'Add'}</Button>
          {/if}
        </div>
      </form>
    </Dialog.Content>

  </Dialog.Portal>

</Dialog.Root>

<div class="mb-4 flex gap-2">
  <Button onclick={() => {
    editingItem = null;
    formData = { title: '', description: '' };
    dialogOpen = true;
  }}>Add New Item</Button>
  
  <Button variant="secondary" onclick={refreshListings} disabled={loading}>
    {loading ? 'Refreshing...' : 'Refresh'}
  </Button>
</div>

<Table.Root>
  <Table.Caption>
    Listing Items Table
  </Table.Caption>

  <Table.Header>
    <Table.Row>
      <Table.Head>ID</Table.Head>
      <Table.Head>Title</Table.Head>
      <Table.Head>Description</Table.Head>
      <Table.Head>Created</Table.Head>
      <Table.Head>Updated</Table.Head>
      <Table.Head>Actions</Table.Head>
    </Table.Row>
  </Table.Header>

  <Table.Body>
    {#each listings as item}
      <Table.Row>
        <Table.Cell>{item.id}</Table.Cell>
        <Table.Cell>{item.title}</Table.Cell>
        <Table.Cell>{item.description}</Table.Cell>
        <Table.Cell>{formatDate(item.created_at)}</Table.Cell>
        <Table.Cell>{formatDate(item.updated_at)}</Table.Cell>
        <Table.Cell>
          <!-- Actions like Edit/Delete can go here -->
          <Button variant="default" onclick={() => startEdit(item)}>Edit</Button>
          <Button variant="destructive" onclick={() => deleteItem(item.id)}>Delete</Button>
        </Table.Cell>
      </Table.Row>
    {/each}
  </Table.Body>
</Table.Root>


            <!-- <th>ID</th>
            <th>Title</th>
            <th>Description</th>
            <th>Created</th>
            <th>Updated</th>
            <th>Actions</th> -->

