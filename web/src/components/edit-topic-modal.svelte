<script lang="ts">
  import { Button } from "$lib/components/ui/button/index";
  import * as Dialog from "$lib/components/ui/dialog/index";
  import { Input } from "$lib/components/ui/input/index";
  import { Label } from "$lib/components/ui/label/index";
  import { Textarea } from "$lib/components/ui/textarea/index";
  import * as Card from "$lib/components/ui/card/index";
  import type { FormSchema } from "$lib/types/json-forms";

  interface Props {
    open: boolean;
    editingTopicId?: number | null;
    initialData?: {
      name: string;
      description: string;
      form_schema?: FormSchema | null;
    };
    loading?: boolean;
    error?: string;
    onClose: () => void;
    onSubmit: (data: { name: string; description: string; form_schema?: FormSchema }) => void | Promise<void>;
  }

  let { 
    open = $bindable(false), 
    editingTopicId = null,
    initialData = { name: '', description: '', form_schema: null },
    loading = false,
    error = '',
    onClose,
    onSubmit
  }: Props = $props();

  let formData = $state({
    name: initialData.name,
    description: initialData.description,
    form_schema_json: initialData.form_schema ? JSON.stringify(initialData.form_schema, null, 2) : ''
  });

  let jsonError = $state('');
  let parsedFormSchema = $state<FormSchema | null>(initialData.form_schema || null);

  // Watch for initialData changes
  $effect(() => {
    formData = {
      name: initialData.name,
      description: initialData.description,
      form_schema_json: initialData.form_schema ? JSON.stringify(initialData.form_schema, null, 2) : ''
    };
    jsonError = '';
    parsedFormSchema = initialData.form_schema || null;
  });

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

  async function handleSubmit(e: Event) {
    e.preventDefault();

    if (!formData.name.trim()) {
      return;
    }
    
    // Validate JSON if provided
    if (formData.form_schema_json.trim()) {
      handleFormSchemaInput();
      if (jsonError) {
        return;
      }
    }
    
    const payload: any = {
      name: formData.name.trim(),
      description: formData.description.trim()
    };
    
    // Add form_schema if provided
    if (formData.form_schema_json.trim() && parsedFormSchema) {
      payload.form_schema = parsedFormSchema;
    }
    
    await onSubmit(payload);
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Content class="max-w-none! w-[85vw] max-h-[90vh] overflow-hidden flex flex-col p-6">
    <Dialog.Header>
      <Dialog.Title>{editingTopicId ? 'Edit Topic' : 'Add New Topic'}</Dialog.Title>
      <Dialog.Description>
        {editingTopicId ? 'Make changes to your topic here.' : 'Add a new topic to your listing.'}
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
        <Button type="button" variant="outline" onclick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button type="submit" disabled={loading}>
          {loading ? 'Saving...' : editingTopicId ? 'Save changes' : 'Add topic'}
        </Button>
      </Dialog.Footer>
    </form>
  </Dialog.Content>
</Dialog.Root>
