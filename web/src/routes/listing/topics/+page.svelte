<script lang="ts">
  import DataTable from "../../../components/data-table.svelte";
  import { Button } from "$lib/components/ui/button/index";
  import * as Dialog from "$lib/components/ui/dialog/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import type { ColumnDef } from "@tanstack/table-core";

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
      const response = await fetch('/api/v1/topics');
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
        const response = await fetch(`/api/v1/topics/${editingTopic}`, {
          method: 'PUT',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            name: formData.name.trim(),
            description: formData.description.trim()
          }),
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
        const response = await fetch('/api/v1/topics', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
          },
          body: JSON.stringify({
            name: formData.name.trim(),
            description: formData.description.trim()
          }),
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
  
  async function deleteTopic(id: number) {
    if (!confirm('Are you sure you want to delete this topic?')) {
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      const response = await fetch(`/api/v1/topics/${id}`, {
        method: 'DELETE',
      });
      
      if (!response.ok) {
        throw new Error('Failed to delete topic');
      }
      
      const topicIndex = topics.findIndex(topic => topic.id === id);
      if (topicIndex !== -1) {
        topics.splice(topicIndex, 1);
        topics = topics;
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
    {#snippet actionsColumn({ row })}
      <div class="flex justify-end gap-2">
        <Button variant="outline" size="sm" onclick={() => openEditDialog(row.original)}>
          Edit
        </Button>
        <Button variant="destructive" size="sm" onclick={() => deleteTopic(row.original.id)}>
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
