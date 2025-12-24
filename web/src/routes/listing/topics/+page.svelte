<script lang="ts">
  import DataTable from "../../../components/data-table.svelte";
  import EditTopicModal from "../../../components/edit-topic-modal.svelte";
  import { Button } from "$lib/components/ui/button/index";
  import * as AlertDialog from "$lib/components/ui/alert-dialog/index";
  import type { ColumnDef } from "@tanstack/table-core";
  import { apiClient, apiPut, apiPost, apiDelete } from "$lib/api";
  import type { Topic, FormSchema } from "$lib/types/json-forms";

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
  let modalInitialData = $state({
    name: '',
    description: '',
    form_schema: null as FormSchema | null
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
    modalInitialData = { name: '', description: '', form_schema: null };
    error = '';
    dialogOpen = true;
  }

  function openEditDialog(topic: Topic) {
    editingTopic = topic.id;
    modalInitialData = { 
      name: topic.name, 
      description: topic.description,
      form_schema: topic.form_schema || null
    };
    error = '';
    dialogOpen = true;
  }

  async function handleSubmit(data: { name: string; description: string; form_schema?: FormSchema }) {
    loading = true;
    error = '';
    
    try {
      if (editingTopic) {
        const response = await apiPut(`/api/v1/topics/${editingTopic}`, data);
        
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
        const response = await apiPost('/api/v1/topics', data);
        
        if (!response.ok) {
          throw new Error('Failed to create topic');
        }
        
        await reloadTopics();
      }
      
      editingTopic = null;
      dialogOpen = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
      throw err; // Re-throw to let the modal handle it
    } finally {
      loading = false;
    }
  }

  function closeDialog() {
    dialogOpen = false;
    editingTopic = null;
    error = '';
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

      await reloadTopics();
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }

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

<EditTopicModal 
  bind:open={dialogOpen}
  editingTopicId={editingTopic}
  initialData={modalInitialData}
  {loading}
  {error}
  onClose={closeDialog}
  onSubmit={handleSubmit}
/>

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
