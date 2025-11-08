<script lang="ts">
  import { goto } from "$app/navigation";
  import DataTable from "../../../../components/data-table.svelte";
  import { Button } from "$lib/components/ui/button/index";
  import * as Dialog from "$lib/components/ui/dialog/index";
  import * as AlertDialog from "$lib/components/ui/alert-dialog/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import type { ColumnDef } from "@tanstack/table-core";

  interface Item {
    id: number;
    title: string;
    description: string;
    topic_id: number;
    created_at: string;
    updated_at: string;
  }

  interface Topic {
    id: number;
    name: string;
    description: string;
    created_at: string;
    updated_at: string;
  }

  interface Props {
    data: {
      topic: Topic;
      items: Item[];
    };
  }

  let { data }: Props = $props();
  
  let items = $state<Item[]>(data.items || []);
  let topic = $state<Topic>(data.topic);
  
  let dialogOpen = $state(false);
  let editingItem = $state<number | null>(null);
  let formData = $state({
    title: '',
    description: ''
  });
  
  let topicDialogOpen = $state(false);
  let topicFormData = $state({
    name: '',
    description: ''
  });
  
  let deleteTopicDialogOpen = $state(false);
  
  let loading = $state(false);
  let error = $state('');

  async function reloadItems() {
    loading = true;
    error = '';
    
    try {
      const response = await fetch(`/api/v1/topics/${topic.id}/items`);
      if (!response.ok) {
        throw new Error('Failed to reload items');
      }
      const data = await response.json();
      items = data.data || [];
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to reload items';
    } finally {
      loading = false;
    }
  }

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
            description: formData.description.trim(),
            topic_id: topic.id
          }),
        });
        
        if (!response.ok) {
          throw new Error('Failed to update item');
        }
        
        const updatedItem = await response.json();
        const itemIndex = items.findIndex(item => item.id === editingItem);
        if (itemIndex !== -1) {
          items[itemIndex] = updatedItem.data;
          items = items;
        }
      } else {
        const response = await fetch('/api/v1/items', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            title: formData.title.trim(),
            description: formData.description.trim(),
            topic_id: topic.id
          }),
        });
        
        if (!response.ok) {
          throw new Error('Failed to create item');
        }
        
        const newItem = await response.json();
        items.push(newItem.data);
        items = items;
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
      
      const itemIndex = items.findIndex(item => item.id === id);
      if (itemIndex !== -1) {
        items.splice(itemIndex, 1);
        items = items;
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

  function openEditTopicDialog() {
    topicFormData = { name: topic.name, description: topic.description };
    topicDialogOpen = true;
  }

  async function handleTopicSubmit(e: Event) {
    e.preventDefault();

    if (!topicFormData.name.trim() || !topicFormData.description.trim()) {
      error = 'Name and description are required';
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      const response = await fetch(`/api/v1/topics/${topic.id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: topicFormData.name.trim(),
          description: topicFormData.description.trim()
        }),
      });
      
      if (!response.ok) {
        throw new Error('Failed to update topic');
      }
      
      const updatedTopic = await response.json();
      topic = updatedTopic.data;
      topicFormData = { name: '', description: '' };
      topicDialogOpen = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }

  async function handleDeleteTopic() {
    loading = true;
    error = '';
    
    try {
      const response = await fetch(`/api/v1/topics/${topic.id}`, {
        method: 'DELETE',
      });
      
      if (!response.ok) {
        throw new Error('Failed to delete topic');
      }
      
      // Navigate back to topics list after successful deletion
      goto('/listing/topics');
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
      deleteTopicDialogOpen = false;
    } finally {
      loading = false;
    }
  }
</script>

<div class="container mx-auto p-6">
  <!-- Topic Header -->
  <div class="mb-8 flex items-start justify-between">
    <div class="flex-1">
      <h3 class="scroll-m-20 text-2xl font-semibold tracking-tight mb-2">
        {topic.name}
      </h3>
      <p class="text-muted-foreground text-base">
        {topic.description}
      </p>
    </div>
    <div class="flex gap-2 ml-4">
      <Button variant="outline" size="sm" onclick={openEditTopicDialog}>
        Edit
      </Button>
      <Button variant="destructive" size="sm" onclick={() => deleteTopicDialogOpen = true}>
        Delete
      </Button>
    </div>
  </div>

  <!-- Items Table -->
  <DataTable 
    data={items} 
    {columns}
    filterPlaceholder="Filter items..."
    showFilter={true}
    showColumnVisibility={true}
    showPagination={true}
    onReload={reloadItems}
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
        {editingItem ? 'Make changes to your item here.' : 'Add a new item to this topic.'}
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

<!-- Edit Topic Dialog -->
<Dialog.Root bind:open={topicDialogOpen}>
  <Dialog.Content class="sm:max-w-[425px]">
    <Dialog.Header>
      <Dialog.Title>Edit Topic</Dialog.Title>
      <Dialog.Description>
        Make changes to your topic here.
      </Dialog.Description>
    </Dialog.Header>

    <form onsubmit={handleTopicSubmit} class="space-y-4">
      {#if error}
        <div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">
          {error}
        </div>
      {/if}

      <div class="space-y-2">
        <Label for="topic-name">Name</Label>
        <Input id="topic-name" type="text" bind:value={topicFormData.name} required />
      </div>

      <div class="space-y-2">
        <Label for="topic-description">Description</Label>
        <Input id="topic-description" type="text" bind:value={topicFormData.description} required />
      </div>

      <Dialog.Footer>
        <Button type="button" variant="outline" onclick={() => { topicDialogOpen = false; }} disabled={loading}>
          Cancel
        </Button>
        <Button type="submit" disabled={loading}>
          {loading ? 'Saving...' : 'Save changes'}
        </Button>
      </Dialog.Footer>
    </form>
  </Dialog.Content>
</Dialog.Root>

<!-- Delete Topic Confirmation Dialog -->
<AlertDialog.Root bind:open={deleteTopicDialogOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Are you absolutely sure?</AlertDialog.Title>
      <AlertDialog.Description>
        This action cannot be undone. This will permanently delete the topic "<strong>{topic.name}</strong>" 
        and all related items ({items.length} item{items.length !== 1 ? 's' : ''}).
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
      <AlertDialog.Action onclick={handleDeleteTopic} disabled={loading}>
        {loading ? 'Deleting...' : 'Delete topic'}
      </AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
