<script lang="ts">
  import { goto } from "$app/navigation";
  import EditTopicModal from "../../../../components/edit-topic-modal.svelte";
  import { Button } from "$lib/components/ui/button/index";
  import * as Card from "$lib/components/ui/card/index";
  import * as Dialog from "$lib/components/ui/dialog/index";
  import * as AlertDialog from "$lib/components/ui/alert-dialog/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import { Textarea } from "$lib/components/ui/textarea/index";
  import { Checkbox } from "$lib/components/ui/checkbox/index";
  import * as Select from "$lib/components/ui/select/index";
  import { Separator } from "$lib/components/ui/separator/index";
  import { apiClient, apiPut, apiPost, apiDelete } from "$lib/api";
  import type { Topic, Item, FormSchema } from "$lib/types/json-forms";

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
  
  // Dynamic form schema values
  let schemaValues = $state<Record<string, any>>({});
  let schemaErrors = $state<Record<string, string>>({});
  
  let topicDialogOpen = $state(false);
  let topicModalInitialData = $state({
    name: '',
    description: '',
    form_schema: null as FormSchema | null
  });
  
  let deleteTopicDialogOpen = $state(false);
  
  let loading = $state(false);
  let error = $state('');
  let searchQuery = $state('');
  let currentPage = $state(1);
  let itemsPerPage = $state(9);

  // Computed filtered items based on search query
  let filteredItems = $derived(
    items.filter(item => 
      searchQuery.trim() === '' || 
      item.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
      item.description.toLowerCase().includes(searchQuery.toLowerCase())
    )
  );

  // Pagination computations
  let totalPages = $derived(Math.ceil(filteredItems.length / itemsPerPage));
  let paginatedItems = $derived(
    filteredItems.slice(
      (currentPage - 1) * itemsPerPage,
      currentPage * itemsPerPage
    )
  );

  // Reset to first page when search query changes
  $effect(() => {
    searchQuery;
    currentPage = 1;
  });

  function goToPage(page: number) {
    if (page >= 1 && page <= totalPages) {
      currentPage = page;
    }
  }

  function changeItemsPerPage(value: number) {
    itemsPerPage = value;
    currentPage = 1;
  }

  async function reloadItems() {
    loading = true;
    error = '';
    
    try {
      const response = await apiClient(`/api/v1/topics/${topic.id}/items`);
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

  function openAddDialog() {
    editingItem = null;
    formData = { title: '', description: '' };
    schemaValues = initializeSchemaValues();
    schemaErrors = {};
    dialogOpen = true;
  }

  function openEditDialog(item: Item) {
    editingItem = item.id;
    formData = { title: item.title, description: item.description };
    
    // Use form_schema_values directly (already parsed as JSON object)
    if (item.form_schema_values) {
      schemaValues = item.form_schema_values;
    } else {
      schemaValues = initializeSchemaValues();
    }
    
    schemaErrors = {};
    dialogOpen = true;
  }
  
  function initializeSchemaValues(): Record<string, any> {
    if (!topic.form_schema?.schema?.properties) {
      return {};
    }
    
    const values: Record<string, any> = {};
    for (const [key, prop] of Object.entries(topic.form_schema.schema.properties)) {
      if (prop.default !== undefined) {
        values[key] = prop.default;
      } else {
        // Initialize with appropriate empty values based on type
        switch (prop.type) {
          case 'string':
            values[key] = '';
            break;
          case 'number':
          case 'integer':
            values[key] = 0;
            break;
          case 'boolean':
            values[key] = false;
            break;
          case 'array':
            values[key] = [];
            break;
          default:
            values[key] = '';
        }
      }
    }
    return values;
  }
  
  function validateSchemaValues(): boolean {
    if (!topic.form_schema?.schema) {
      return true;
    }
    
    schemaErrors = {};
    let isValid = true;
    
    const requiredFields = topic.form_schema.schema.required || [];
    
    for (const [key, prop] of Object.entries(topic.form_schema.schema.properties)) {
      const value = schemaValues[key];
      
      // Check required fields
      if (requiredFields.includes(key)) {
        if (value === undefined || value === null || value === '') {
          schemaErrors[key] = `${prop.title || key} is required`;
          isValid = false;
          continue;
        }
      }
      
      // Type validation
      if (value !== undefined && value !== null && value !== '') {
        switch (prop.type) {
          case 'number':
          case 'integer':
            if (isNaN(Number(value))) {
              schemaErrors[key] = `${prop.title || key} must be a valid number`;
              isValid = false;
            }
            break;
        }
      }
    }
    
    return isValid;
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();

    if (!formData.title.trim() || !formData.description.trim()) {
      error = 'Title and description are required';
      return;
    }
    
    // Validate schema values if form_schema exists
    if (topic.form_schema && !validateSchemaValues()) {
      error = 'Please fix the validation errors in the form';
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      const payload: any = {
        title: formData.title.trim(),
        description: formData.description.trim(),
        topic_id: topic.id
      };
      
      // Add form_schema_values if schema exists
      if (topic.form_schema?.schema?.properties) {
        payload.form_schema_values = schemaValues;
      }
      
      if (editingItem) {
        const response = await apiPut(`/api/v1/topics/${topic.id}/items/${editingItem}`, payload);
        
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
        const response = await apiPost(`/api/v1/topics/${topic.id}/items`, payload);
        
        if (!response.ok) {
          console.log(response);
          throw new Error('Failed to create item');
        }
        
        const newItem = await response.json();
        items.push(newItem.data);
        items = items;
      }
      
      formData = { title: '', description: '' };
      schemaValues = {};
      schemaErrors = {};
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
      const response = await apiDelete(`/api/v1/topics/${topic.id}/items/${id}`);
      
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
    topicModalInitialData = { 
      name: topic.name, 
      description: topic.description,
      form_schema: topic.form_schema || null
    };
    error = '';
    topicDialogOpen = true;
  }

  async function handleTopicSubmit(data: { name: string; description: string; form_schema?: FormSchema }) {
    loading = true;
    error = '';
    
    try {
      const response = await apiPut(`/api/v1/topics/${topic.id}`, data);
      
      if (!response.ok) {
        throw new Error('Failed to update topic');
      }
      
      const updatedTopic = await response.json();
      topic = updatedTopic.data;
      topicDialogOpen = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
      throw err; // Re-throw to let the modal handle it
    } finally {
      loading = false;
    }
  }

  function closeTopicDialog() {
    topicDialogOpen = false;
    error = '';
  }

  async function handleDeleteTopic() {
    loading = true;
    error = '';
    
    try {
      const response = await apiDelete(`/api/v1/topics/${topic.id}`);
      
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
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-4">
      <div class="flex-1 max-w-sm">
        <Input 
          type="text" 
          placeholder="Filter items..." 
          bind:value={searchQuery}
          class="w-full"
        />
      </div>
      <div class="flex gap-2">
        <Button variant="outline" size="sm" onclick={reloadItems} disabled={loading}>
          {loading ? 'Reloading...' : 'Reload'}
        </Button>
        <Button size="sm" onclick={openAddDialog}>
          Add New Item
        </Button>
      </div>
    </div>

    {#if error}
      <div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive">
        {error}
      </div>
    {/if}

    {#if filteredItems.length === 0}
      <div class="text-center py-12 border rounded-lg bg-muted/20">
        <p class="text-muted-foreground">
          {searchQuery ? 'No items match your search.' : 'No items yet. Click "Add New Item" to create one.'}
        </p>
      </div>
    {:else}
      <div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {#each paginatedItems as item (item.id)}
          <Card.Root class="hover:shadow-md transition-shadow">
            <Card.Header>
              <div class="flex items-start justify-between">
                <div class="flex-1">
                  <Card.Title class="line-clamp-1">{item.title}</Card.Title>
                  <Card.Description class="line-clamp-2 mt-1.5">
                    {item.description}
                  </Card.Description>
                </div>
              </div>
            </Card.Header>
            
            {#if item.form_schema_values && Object.keys(item.form_schema_values).length > 0}
              <Card.Content class="space-y-3">
                <Separator />
                <div class="space-y-2">
                  {#each Object.entries(item.form_schema_values) as [key, value]}
                    {@const prop = topic.form_schema?.schema?.properties?.[key]}
                    <div class="text-sm">
                      <span class="font-medium text-muted-foreground">
                        {prop?.title || key}:
                      </span>
                      <span class="ml-2">
                        {#if typeof value === 'boolean'}
                          {value ? 'Yes' : 'No'}
                        {:else if value === null || value === undefined || value === ''}
                          <span class="text-muted-foreground italic">Not set</span>
                        {:else if Array.isArray(value)}
                          {value.join(', ')}
                        {:else if prop?.format === 'uri' || prop?.format === 'url'}
                          <a href={String(value)} target="_blank" rel="noopener noreferrer" class="text-blue-500 hover:underline break-all">{String(value)}</a>
                        {:else}
                          {value}
                        {/if}
                      </span>
                    </div>
                  {/each}
                </div>
              </Card.Content>
            {/if}
            
            <Card.Footer class="flex justify-between items-center">
              <div class="text-xs text-muted-foreground">
                Updated: {formatDate(item.updated_at)}
              </div>
              <div class="flex gap-2">
                <Button variant="outline" size="sm" onclick={() => openEditDialog(item)}>
                  Edit
                </Button>
                <Button variant="destructive" size="sm" onclick={() => deleteItem(item.id)}>
                  Delete
                </Button>
              </div>
            </Card.Footer>
          </Card.Root>
        {/each}
      </div>

      <!-- Pagination Controls -->
      <div class="flex flex-col sm:flex-row items-center justify-between gap-4 pt-4">
        <div class="flex items-center gap-2">
          <span class="text-sm text-muted-foreground">Items per page:</span>
          <Select.Root
            type="single"
            value={itemsPerPage.toString()}
            onValueChange={(v) => v && changeItemsPerPage(Number(v))}
          >
            <Select.Trigger class="w-20">
              {itemsPerPage}
            </Select.Trigger>
            <Select.Content>
              <Select.Item value="9" label="9">9</Select.Item>
              <Select.Item value="18" label="18">18</Select.Item>
              <Select.Item value="36" label="36">36</Select.Item>
              <Select.Item value="50" label="50">50</Select.Item>
            </Select.Content>
          </Select.Root>
        </div>

        <div class="text-sm text-muted-foreground">
          Showing {filteredItems.length === 0 ? 0 : (currentPage - 1) * itemsPerPage + 1} to {Math.min(currentPage * itemsPerPage, filteredItems.length)} of {filteredItems.length} item{filteredItems.length !== 1 ? 's' : ''}
        </div>

        <div class="flex items-center gap-2">
          <Button 
            variant="outline" 
            size="sm" 
            onclick={() => goToPage(currentPage - 1)}
            disabled={currentPage === 1 || totalPages === 0}
          >
            Previous
          </Button>
          
          <div class="flex items-center gap-1">
            {#if totalPages <= 7}
              {#each Array.from({ length: totalPages }, (_, i) => i + 1) as page}
                <Button
                  variant={currentPage === page ? 'default' : 'outline'}
                  size="sm"
                  onclick={() => goToPage(page)}
                  class="w-10"
                >
                  {page}
                </Button>
              {/each}
            {:else}
              {#if currentPage <= 3}
                {#each [1, 2, 3, 4] as page}
                  <Button
                    variant={currentPage === page ? 'default' : 'outline'}
                    size="sm"
                    onclick={() => goToPage(page)}
                    class="w-10"
                  >
                    {page}
                  </Button>
                {/each}
                <span class="px-2 text-muted-foreground">...</span>
                <Button
                  variant="outline"
                  size="sm"
                  onclick={() => goToPage(totalPages)}
                  class="w-10"
                >
                  {totalPages}
                </Button>
              {:else if currentPage >= totalPages - 2}
                <Button
                  variant="outline"
                  size="sm"
                  onclick={() => goToPage(1)}
                  class="w-10"
                >
                  1
                </Button>
                <span class="px-2 text-muted-foreground">...</span>
                {#each [totalPages - 3, totalPages - 2, totalPages - 1, totalPages] as page}
                  <Button
                    variant={currentPage === page ? 'default' : 'outline'}
                    size="sm"
                    onclick={() => goToPage(page)}
                    class="w-10"
                  >
                    {page}
                  </Button>
                {/each}
              {:else}
                <Button
                  variant="outline"
                  size="sm"
                  onclick={() => goToPage(1)}
                  class="w-10"
                >
                  1
                </Button>
                <span class="px-2 text-muted-foreground">...</span>
                {#each [currentPage - 1, currentPage, currentPage + 1] as page}
                  <Button
                    variant={currentPage === page ? 'default' : 'outline'}
                    size="sm"
                    onclick={() => goToPage(page)}
                    class="w-10"
                  >
                    {page}
                  </Button>
                {/each}
                <span class="px-2 text-muted-foreground">...</span>
                <Button
                  variant="outline"
                  size="sm"
                  onclick={() => goToPage(totalPages)}
                  class="w-10"
                >
                  {totalPages}
                </Button>
              {/if}
            {/if}
          </div>

          <Button 
            variant="outline" 
            size="sm" 
            onclick={() => goToPage(currentPage + 1)}
            disabled={currentPage === totalPages || totalPages === 0}
          >
            Next
          </Button>
        </div>
      </div>
    {/if}
  </div>
</div>

<Dialog.Root bind:open={dialogOpen}>
  <Dialog.Content class="sm:max-w-[600px] max-h-[90vh] overflow-hidden flex flex-col">
    <Dialog.Header>
      <Dialog.Title>{editingItem ? 'Edit Item' : 'Add New Item'}</Dialog.Title>
      <Dialog.Description>
        {editingItem ? 'Make changes to your item here.' : 'Add a new item to this topic.'}
      </Dialog.Description>
    </Dialog.Header>

    <form onsubmit={handleSubmit} class="flex-1 overflow-hidden flex flex-col">
      {#if error}
        <div class="rounded-md bg-destructive/15 p-3 text-sm text-destructive mb-4">
          {error}
        </div>
      {/if}

      <div class="flex-1 overflow-y-auto space-y-4 pr-2">
        <div class="space-y-2">
          <Label for="title">Title</Label>
          <Input id="title" type="text" bind:value={formData.title} required />
        </div>

        <div class="space-y-2">
          <Label for="description">Description</Label>
          <Input id="description" type="text" bind:value={formData.description} required />
        </div>

        {#if topic.form_schema?.schema?.properties}
          <div class="border-t pt-4 mt-4">
            <h4 class="font-semibold mb-4 text-sm">Additional Fields</h4>
            <div class="space-y-4">
              {#each Object.entries(topic.form_schema.schema.properties) as [key, prop]}
                {@const isRequired = topic.form_schema.schema.required?.includes(key)}
                <div class="space-y-2">
                  <Label for={key} class="flex items-center gap-2">
                    {prop.title || key}
                    {#if isRequired}
                      <span class="text-xs text-destructive">*</span>
                    {/if}
                  </Label>
                  
                  {#if prop.description}
                    <p class="text-xs text-muted-foreground">{prop.description}</p>
                  {/if}
                  
                  {#if prop.enum && prop.enum.length > 0}
                    <!-- Select dropdown for enum values -->
                    <Select.Root
                      type="single"
                      bind:value={schemaValues[key]}
                    >
                      <Select.Trigger class="w-full">
                        {schemaValues[key] || 'Select ' + (prop.title || key)}
                      </Select.Trigger>
                      <Select.Content>
                        {#each prop.enum as option}
                          <Select.Item value={option} label={option}>
                            {option}
                          </Select.Item>
                        {/each}
                      </Select.Content>
                    </Select.Root>
                  {:else if prop.type === 'boolean'}
                    <!-- Checkbox for boolean -->
                    <div class="flex items-center space-x-2">
                      <Checkbox 
                        id={key}
                        checked={schemaValues[key] || false}
                        onCheckedChange={(checked) => {
                          schemaValues[key] = checked === true;
                        }}
                      />
                      <Label for={key} class="text-sm font-normal">
                        {prop.title || key}
                      </Label>
                    </div>
                  {:else if prop.type === 'number' || prop.type === 'integer'}
                    <!-- Number input -->
                    <Input 
                      id={key}
                      type="number"
                      step={prop.type === 'integer' ? '1' : 'any'}
                      bind:value={schemaValues[key]}
                      required={isRequired}
                    />
                  {:else if prop.format === 'date'}
                    <!-- Date input -->
                    <Input 
                      id={key}
                      type="date"
                      bind:value={schemaValues[key]}
                      required={isRequired}
                    />
                  {:else if prop.format === 'date-time'}
                    <!-- DateTime input -->
                    <Input 
                      id={key}
                      type="datetime-local"
                      bind:value={schemaValues[key]}
                      required={isRequired}
                    />
                  {:else if prop.format === 'email'}
                    <!-- Email input -->
                    <Input 
                      id={key}
                      type="email"
                      bind:value={schemaValues[key]}
                      required={isRequired}
                    />
                  {:else if prop.format === 'uri' || prop.format === 'url'}
                    <!-- URL input -->
                    <Input 
                      id={key}
                      type="url"
                      bind:value={schemaValues[key]}
                      required={isRequired}
                    />
                  {:else}
                    <!-- Default text input or textarea -->
                    {#if prop.description && prop.description.length > 50}
                      <Textarea 
                        id={key}
                        bind:value={schemaValues[key]}
                        rows={3}
                        required={isRequired}
                      />
                    {:else}
                      <Input 
                        id={key}
                        type="text"
                        bind:value={schemaValues[key]}
                        required={isRequired}
                      />
                    {/if}
                  {/if}
                  
                  {#if schemaErrors[key]}
                    <div class="text-xs text-destructive">
                      {schemaErrors[key]}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          </div>
        {/if}
      </div>

      <Dialog.Footer class="mt-4 pt-4 border-t">
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

<EditTopicModal 
  bind:open={topicDialogOpen}
  editingTopicId={topic.id}
  initialData={topicModalInitialData}
  {loading}
  {error}
  onClose={closeTopicDialog}
  onSubmit={handleTopicSubmit}
/>

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
