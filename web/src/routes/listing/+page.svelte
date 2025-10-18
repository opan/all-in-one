<script lang="ts">
  import * as Table from "$lib/components/ui/table/index"
  import { Button } from "$lib/components/ui/button/index"
  import * as Dialog from "$lib/components/ui/dialog/index"
  import * as Card from "$lib/components/ui/card/index"
  import * as Sidebar from "$lib/components/ui/sidebar/index"
  import * as Breadcrumb from "$lib/components/ui/breadcrumb/index"
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import { Separator } from "$lib/components/ui/separator/index";
  import ThemeToggle from "$lib/components/theme-toggle.svelte";
  import AppSidebar from "$lib/components/app-sidebar.svelte";

  interface Item {
    id: number;
    title: string;
    description: string;
    created_at: string;
    updated_at: string;
  }

  // Dummy data for now
  let listings = $state<Item[]>([
    { id: 1, title: "First Item", description: "Description for first item", created_at: "2025-10-15T10:00:00Z", updated_at: "2025-10-15T10:00:00Z" },
    { id: 2, title: "Second Item", description: "Description for second item", created_at: "2025-10-16T11:30:00Z", updated_at: "2025-10-16T11:30:00Z" },
    { id: 3, title: "Third Item", description: "Description for third item", created_at: "2025-10-17T09:15:00Z", updated_at: "2025-10-17T09:15:00Z" },
    { id: 4, title: "Fourth Item", description: "Description for fourth item", created_at: "2025-10-18T14:20:00Z", updated_at: "2025-10-18T14:20:00Z" },
  ]);
  
  // Form state
  let dialogOpen = $state(false);
  let editingItem = $state<number | null>(null);
  let formData = $state({
    title: '',
    description: ''
  });
  
  // Loading and error states
  let loading = $state(false);
  let error = $state('');

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

  
  // Add new item
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
        // Update existing item
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
        // Create new item
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
      
      // Reset form
      formData = { title: '', description: '' };
      editingItem = null;
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

<Sidebar.Provider>
  <AppSidebar />
  
  <Sidebar.Inset>
    <!-- Top Navigation Bar -->
    <header class="flex h-14 shrink-0 items-center gap-2 border-b px-4">
      <Sidebar.Trigger class="-ml-1" />
      <Separator orientation="vertical" class="mr-2 h-4" />
      
      <Breadcrumb.Root>
        <Breadcrumb.List>
          <Breadcrumb.Item>
            <Breadcrumb.Link href="/">Building Your Application</Breadcrumb.Link>
          </Breadcrumb.Item>
          <Breadcrumb.Separator />
          <Breadcrumb.Item>
            <Breadcrumb.Page>Data Fetching</Breadcrumb.Page>
          </Breadcrumb.Item>
        </Breadcrumb.List>
      </Breadcrumb.Root>
      
      <div class="ml-auto flex items-center gap-2">
        <ThemeToggle />
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="flex-1 overflow-auto">
      <div class="container mx-auto p-6">
        <Card.Root class="min-h-[calc(100vh-8rem)]">
          <Card.Header class="border-b">
            <div class="flex items-center justify-between">
              <Card.Title class="text-2xl">TABLE</Card.Title>
              <Button onclick={openAddDialog}>Add New Item</Button>
            </div>
          </Card.Header>
          <Card.Content class="p-6">
            <div class="rounded-md border">
              <Table.Root>
                <Table.Header>
                  <Table.Row>
                    <Table.Head class="w-[80px]">ID</Table.Head>
                    <Table.Head>Title</Table.Head>
                    <Table.Head>Description</Table.Head>
                    <Table.Head class="w-[180px]">Created</Table.Head>
                    <Table.Head class="w-[180px]">Updated</Table.Head>
                    <Table.Head class="w-[180px] text-right">Actions</Table.Head>
                  </Table.Row>
                </Table.Header>

                <Table.Body>
                  {#each listings as item}
                    <Table.Row>
                      <Table.Cell class="font-medium">{item.id}</Table.Cell>
                      <Table.Cell>{item.title}</Table.Cell>
                      <Table.Cell>{item.description}</Table.Cell>
                      <Table.Cell class="text-muted-foreground">{formatDate(item.created_at)}</Table.Cell>
                      <Table.Cell class="text-muted-foreground">{formatDate(item.updated_at)}</Table.Cell>
                      <Table.Cell class="text-right">
                        <div class="flex justify-end gap-2">
                          <Button variant="outline" size="sm" onclick={() => openEditDialog(item)}>Edit</Button>
                          <Button variant="destructive" size="sm" onclick={() => deleteItem(item.id)}>Delete</Button>
                        </div>
                      </Table.Cell>
                    </Table.Row>
                  {/each}
                </Table.Body>
              </Table.Root>
            </div>
          </Card.Content>
        </Card.Root>
      </div>
    </main>
  </Sidebar.Inset>
</Sidebar.Provider>

<!-- Add/Edit Dialog -->
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

