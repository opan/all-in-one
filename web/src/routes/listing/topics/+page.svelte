<script lang="ts">
  import DataTable from "../../../components/data-table.svelte";
  import { Button } from "$lib/components/ui/button/index";
  import * as Dialog from "$lib/components/ui/dialog/index";
  import * as AlertDialog from "$lib/components/ui/alert-dialog/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import type { ColumnDef } from "@tanstack/table-core";
  import { apiClient, apiPut, apiPost, apiDelete } from "$lib/api";

  interface Topic {
    id: number;
    name: string;
    description: string;
    created_at: string;
    updated_at: string;
  }

  interface Props {
    data: {
      topics: Topic[];
    };
  }

  let { data }: Props = $props();
  
  let topics = $state<Topic[]>(data.topics || []);
  
  let dialogOpen = $state(false);
  let deleteDialogOpen = $state(false);
  let topicToDelete = $state<Topic | null>(null);
  let editingTopic = $state<number | null>(null);
  let formData = $state({
    name: '',
    description: ''
  });
  
  let loading = $state(false);
  let error = $state('');

  async function reloadTopics() {
    loading = true;
    error = '';
    
    try {
      const response = await apiClient('/api/v1/topics');
      if (!response.ok) {
        throw new Error('Failed to reload topics');
      }
      const data = await response.json();
      topics = data.data || [];
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to reload topics';
    } finally {
      loading = false;
    }
  }

  const columns: ColumnDef<Topic>[] = [
    {
      accessorKey: "id",
      header: "ID",
      cell: (info) => info.getValue(),
      enableHiding: true,
    },
    {
      accessorKey: "name",
      header: "Name",
      cell: (info) => {
        const topic = info.row.original;
        return {
          render: () => null,
          data: topic
        };
      },
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
        const topic = info.row.original;
        return {
          render: () => null,
          data: topic
        };
      },
      enableHiding: false,
    },
  ];

  function openAddDialog() {
    editingTopic = null;
    formData = { name: '', description: '' };
    dialogOpen = true;
  }

  function openEditDialog(topic: Topic) {
    editingTopic = topic.id;
    formData = { name: topic.name, description: topic.description };
    dialogOpen = true;
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();

    if (!formData.name.trim()) {
      error = 'Name is required';
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      if (editingTopic) {
        const response = await apiPut(`/api/v1/topics/${editingTopic}`, {
          name: formData.name.trim(),
          description: formData.description.trim()
        });
        
        if (!response.ok) {
          throw new Error('Failed to update topic');
        }
        
        const updatedTopic = await response.json();
        const topicIndex = topics.findIndex(topic => topic.id === editingTopic);
        if (topicIndex !== -1) {
          topics[topicIndex] = updatedTopic.data;
          topics = topics;
        }
      } else {
        const response = await apiPost('/api/v1/topics', {
          name: formData.name.trim(),
          description: formData.description.trim()
        });
        
        if (!response.ok) {
          throw new Error('Failed to create topic');
        }
        
        const newTopic = await response.json();
        topics.push(newTopic.data);
        topics = topics;
      }
      
      formData = { name: '', description: '' };
      editingTopic = null;
      dialogOpen = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }
  
  function openDeleteDialog(topic: Topic) {
    topicToDelete = topic;
    deleteDialogOpen = true;
  }

  async function confirmDelete() {
    if (!topicToDelete) return;
    
    loading = true;
    error = '';
    
    try {
      const response = await apiDelete(`/api/v1/topics/${topicToDelete.id}`);
      
      if (!response.ok) {
        throw new Error('Failed to delete topic');
      }
      
      const topicIndex = topics.findIndex(topic => topic.id === topicToDelete!.id);
      if (topicIndex !== -1) {
        topics.splice(topicIndex, 1);
        topics = topics;
      }
      
      deleteDialogOpen = false;
      topicToDelete = null;
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
    data={topics} 
    {columns}
    filterPlaceholder="Filter topics..."
    showFilter={true}
    showColumnVisibility={true}
    showPagination={true}
    onReload={reloadTopics}
    actions={[
      {
        label: "Add New Topic",
        onclick: openAddDialog
      }
    ]}
  >
    {#snippet nameColumn({ row })}
      <a href="/listing/topics/{row.original.id}" class="text-blue-600 hover:underline cursor-pointer">
        {row.original.name}
      </a>
    {/snippet}
    {#snippet actionsColumn({ row })}
      <div class="flex justify-end gap-2">
        <Button variant="outline" size="sm" onclick={() => openEditDialog(row.original)}>
          Edit
        </Button>
        <Button variant="destructive" size="sm" onclick={() => openDeleteDialog(row.original)}>
          Delete
        </Button>
      </div>
    {/snippet}
  </DataTable>
</div>

<Dialog.Root bind:open={dialogOpen}>
  <Dialog.Content class="sm:max-w-[425px]">
    <Dialog.Header>
      <Dialog.Title>{editingTopic ? 'Edit Topic' : 'Add New Topic'}</Dialog.Title>
      <Dialog.Description>
        {editingTopic ? 'Make changes to your topic here.' : 'Add a new topic to your listing.'}
      </Dialog.Description>
    </Dialog.Header>

    <form onsubmit={handleSubmit} class="space-y-4">
      {#if error}
        <div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">
          {error}
        </div>
      {/if}

      <div class="space-y-2">
        <Label for="name">Name</Label>
        <Input id="name" type="text" bind:value={formData.name} required />
      </div>

      <div class="space-y-2">
        <Label for="description">Description</Label>
        <Input id="description" type="text" bind:value={formData.description} />
      </div>

      <Dialog.Footer>
        <Button type="button" variant="outline" onclick={() => { dialogOpen = false; editingTopic = null; }} disabled={loading}>
          Cancel
        </Button>
        <Button type="submit" disabled={loading}>
          {loading ? 'Saving...' : editingTopic ? 'Save changes' : 'Add topic'}
        </Button>
      </Dialog.Footer>
    </form>
  </Dialog.Content>
</Dialog.Root>

<AlertDialog.Root bind:open={deleteDialogOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Delete Topic</AlertDialog.Title>
      <AlertDialog.Description>
        {#if topicToDelete}
          <div class="space-y-3">
            <p>Are you sure you want to delete <span class="font-semibold">"{topicToDelete.name}"</span>?</p>
            <div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive border border-destructive/20">
              <p class="font-semibold mb-1">⚠️ Warning</p>
              <p>This action will delete the topic and all related items associated with it. This cannot be undone.</p>
            </div>
          </div>
        {/if}
        
        {#if error}
          <div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive mt-3">
            {error}
          </div>
        {/if}
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel 
        onclick={() => { deleteDialogOpen = false; topicToDelete = null; error = ''; }} 
        disabled={loading}
      >
        Cancel
      </AlertDialog.Cancel>
      <AlertDialog.Action 
        onclick={confirmDelete} 
        disabled={loading}
        class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
      >
        {loading ? 'Deleting...' : 'Delete'}
      </AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
