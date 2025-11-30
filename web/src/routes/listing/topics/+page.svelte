<script lang="ts">
  import DataTable from "../../../components/data-table.svelte";
  import { Button } from "$lib/components/ui/button/index";
  import * as Dialog from "$lib/components/ui/dialog/index";
  import * as AlertDialog from "$lib/components/ui/alert-dialog/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import { Textarea } from "$lib/components/ui/textarea/index";
  import * as Card from "$lib/components/ui/card/index";
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
  let formData = $state({
    name: '',
    description: '',
    form_schema_json: ''
  });
  
  let loading = $state(false);
  let error = $state('');
  let jsonError = $state('');
  let parsedFormSchema = $state<FormSchema | null>(null);

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
    formData = { name: '', description: '', form_schema_json: '' };
    jsonError = '';
    parsedFormSchema = null;
    dialogOpen = true;
  }

  function openEditDialog(topic: Topic) {
    editingTopic = topic.id;
    formData = { 
      name: topic.name, 
      description: topic.description,
      form_schema_json: topic.form_schema ? JSON.stringify(topic.form_schema, null, 2) : ''
    };
    jsonError = '';
    parsedFormSchema = topic.form_schema || null;
    dialogOpen = true;
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();

    if (!formData.name.trim()) {
      error = 'Name is required';
      return;
    }
    
    // Validate JSON if provided
    if (formData.form_schema_json.trim()) {
      handleFormSchemaInput();
      if (jsonError) {
        error = 'Please fix the JSON schema errors before saving';
        return;
      }
    }
    
    loading = true;
    error = '';
    
    try {
      const payload: any = {
        name: formData.name.trim(),
        description: formData.description.trim()
      };
      
      // Add form_schema if provided
      if (formData.form_schema_json.trim() && parsedFormSchema) {
        payload.form_schema = parsedFormSchema;
      }
      
      if (editingTopic) {
        const response = await apiPut(`/api/v1/topics/${editingTopic}`, payload);
        
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
        const response = await apiPost('/api/v1/topics', payload);
        
        if (!response.ok) {
          throw new Error('Failed to create topic');
        }
        
        await reloadTopics();
      }
      
      formData = { name: '', description: '', form_schema_json: '' };
      jsonError = '';
      parsedFormSchema = null;
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

      await reloadTopics();
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }

  function handleFormSchemaInput() {
    jsonError = '';
    parsedFormSchema = null;
    
    if (!formData.form_schema_json.trim()) {
      return;
    }
    
    try {
      const parsed = JSON.parse(formData.form_schema_json);
      
      // Validate JSONForms structure
      if (!parsed.schema || typeof parsed.schema !== 'object') {
        jsonError = 'Invalid schema: must have a "schema" object (JSON Schema)';
        return;
      }
      
      if (!parsed.uischema || typeof parsed.uischema !== 'object') {
        jsonError = 'Invalid schema: must have a "uischema" object (UI Schema)';
        return;
      }
      
      // Validate JSON Schema structure
      if (parsed.schema.type !== 'object') {
        jsonError = 'Invalid schema: schema.type must be "object"';
        return;
      }
      
      if (!parsed.schema.properties || typeof parsed.schema.properties !== 'object') {
        jsonError = 'Invalid schema: schema must have "properties" object';
        return;
      }
      
      // Validate UI Schema structure
      const validUITypes = ['Control', 'VerticalLayout', 'HorizontalLayout', 'Group'];
      if (!parsed.uischema.type || !validUITypes.includes(parsed.uischema.type)) {
        jsonError = `Invalid uischema: type must be one of ${validUITypes.join(', ')}`;
        return;
      }
      
      if (parsed.uischema.type !== 'Control' && (!parsed.uischema.elements || !Array.isArray(parsed.uischema.elements))) {
        jsonError = 'Invalid uischema: layout elements must have an "elements" array';
        return;
      }
      
      parsedFormSchema = parsed;
    } catch (e) {
      jsonError = e instanceof Error ? e.message : 'Invalid JSON';
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
  <Dialog.Content class="max-w-none! w-[85vw] max-h-[90vh] overflow-hidden flex flex-col p-6">
    <Dialog.Header>
      <Dialog.Title>{editingTopic ? 'Edit Topic' : 'Add New Topic'}</Dialog.Title>
      <Dialog.Description>
        {editingTopic ? 'Make changes to your topic here.' : 'Add a new topic to your listing.'}
      </Dialog.Description>
    </Dialog.Header>

    <form onsubmit={handleSubmit} class="flex-1 overflow-hidden flex flex-col gap-4">
      {#if error}
        <div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">
          {error}
        </div>
      {/if}

      <!-- Two column layout: Form inputs on left, Preview on right -->
      <div class="grid grid-cols-2 gap-6 flex-1 overflow-hidden">
        <!-- Left Column: Form Inputs -->
        <div class="space-y-4 overflow-y-auto pr-2">
          <div class="space-y-2">
            <Label for="name">Name</Label>
            <Input id="name" type="text" bind:value={formData.name} required />
          </div>

          <div class="space-y-2">
            <Label for="description">Description</Label>
            <Input id="description" type="text" bind:value={formData.description} />
          </div>

          <div class="space-y-2">
            <Label for="form_schema">Form Schema (JSON)</Label>
            <Textarea 
              id="form_schema" 
              bind:value={formData.form_schema_json}
              oninput={handleFormSchemaInput}
              placeholder={`{
  "schema": {
    "type": "object",
    "properties": {
      "title": {
        "type": "string",
        "title": "Title"
      },
      "price": {
        "type": "number",
        "title": "Price"
      }
    },
    "required": ["title"]
  },
  "uischema": {
    "type": "VerticalLayout",
    "elements": [
      {
        "type": "Control",
        "scope": "#/properties/title"
      },
      {
        "type": "Control",
        "scope": "#/properties/price"
      }
    ]
  }
}`}
              rows={20}
              class="font-mono text-sm resize-none"
            />
            {#if jsonError}
              <div class="rounded-md bg-destructive/15 p-2 text-xs text-destructive">
                {jsonError}
              </div>
            {/if}
            <p class="text-xs text-muted-foreground">
              Define custom fields using JSONForms.io format (schema + uischema). Leave empty for basic items.
              <a href="https://jsonforms.io/docs/uischema" target="_blank" class="text-blue-600 hover:underline">Learn more</a>
            </p>
          </div>
        </div>

        <!-- Right Column: Live Preview -->
        <div class="border-l pl-6 overflow-y-auto">
          <div class="sticky top-0 bg-background pb-2 mb-4 border-b">
            <h3 class="font-semibold text-lg">Live Preview</h3>
            <p class="text-sm text-muted-foreground">Preview of the form schema</p>
          </div>

          {#if parsedFormSchema && parsedFormSchema.schema.properties}
            <div class="space-y-4">
              <Card.Root>
                <Card.Header>
                  <Card.Title class="text-sm flex items-center gap-2">
                    <span class="text-lg">📋</span>
                    JSON Schema Properties
                  </Card.Title>
                  <p class="text-xs text-muted-foreground">
                    {Object.keys(parsedFormSchema.schema.properties).length} field(s) defined
                  </p>
                </Card.Header>
                <Card.Content class="space-y-3">
                  {#each Object.entries(parsedFormSchema.schema.properties) as [key, prop]}
                    <div class="border-b pb-3 last:border-b-0">
                      <div class="flex items-start justify-between">
                        <div class="flex-1">
                          <div class="flex items-center gap-2">
                            <span class="font-medium text-sm">{prop.title || key}</span>
                            {#if parsedFormSchema.schema.required?.includes(key)}
                              <span class="text-xs bg-destructive/20 text-destructive px-1.5 py-0.5 rounded">Required</span>
                            {/if}
                          </div>
                          <div class="text-xs text-muted-foreground mt-1 flex flex-wrap gap-2">
                            <span class="bg-muted px-1.5 py-0.5 rounded">Key: {key}</span>
                            <span class="bg-muted px-1.5 py-0.5 rounded">Type: {prop.type}</span>
                            {#if prop.format}
                              <span class="bg-muted px-1.5 py-0.5 rounded">Format: {prop.format}</span>
                            {/if}
                          </div>
                          {#if prop.description}
                            <p class="text-xs text-muted-foreground mt-2 italic">{prop.description}</p>
                          {/if}
                          {#if prop.enum && prop.enum.length > 0}
                            <div class="text-xs mt-2">
                              <span class="font-medium">Options:</span>
                              <div class="flex flex-wrap gap-1 mt-1">
                                {#each prop.enum as option}
                                  <span class="bg-primary/10 text-primary px-2 py-0.5 rounded text-xs">{option}</span>
                                {/each}
                              </div>
                            </div>
                          {/if}
                          {#if prop.default !== undefined && prop.default !== null}
                            <div class="text-xs text-muted-foreground mt-2">
                              <span class="font-medium">Default:</span> {JSON.stringify(prop.default)}
                            </div>
                          {/if}
                        </div>
                      </div>
                    </div>
                  {/each}
                </Card.Content>
              </Card.Root>

              <Card.Root>
                <Card.Header>
                  <Card.Title class="text-sm flex items-center gap-2">
                    <span class="text-lg">🎨</span>
                    UI Schema Layout
                  </Card.Title>
                  <p class="text-xs text-muted-foreground">
                    Layout type: {parsedFormSchema.uischema.type}
                  </p>
                </Card.Header>
                <Card.Content>
                  <div class="text-xs font-mono bg-muted p-3 rounded overflow-auto max-h-60">
                    {JSON.stringify(parsedFormSchema.uischema, null, 2)}
                  </div>
                </Card.Content>
              </Card.Root>
            </div>
          {:else}
            <div class="flex flex-col items-center justify-center h-64 text-center text-muted-foreground">
              <div class="text-5xl mb-4">📝</div>
              <p class="text-sm font-medium">No schema defined yet</p>
              <p class="text-xs mt-2">Enter a valid JSONForms schema on the left to see the preview</p>
            </div>
          {/if}
        </div>
      </div>

      <Dialog.Footer class="shrink-0 pt-4 border-t">
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
