<script lang="ts">
  import { Button } from '$lib/components/ui/button';
  import { Input } from '$lib/components/ui/input';
  import { Label } from '$lib/components/ui/label';
  import { Checkbox } from '$lib/components/ui/checkbox';
  import type { CustomField, FieldType } from '$lib/types/topic';

  interface Props {
    fields?: CustomField[];
    onchange?: (fields: CustomField[]) => void;
  }

  let { fields = $bindable([]), onchange }: Props = $props();

  function addField() {
    fields = [...fields, {
      key: '',
      label: '',
      type: 'text',
      required: false,
      options: []
    }];
    onchange?.(fields);
  }

  function removeField(index: number) {
    fields = fields.filter((_, i) => i !== index);
    onchange?.(fields);
  }

  function updateField(index: number, updates: Partial<CustomField>) {
    fields = fields.map((field, i) => 
      i === index ? { ...field, ...updates } : field
    );
    onchange?.(fields);
  }

  function addOption(index: number) {
    const field = fields[index];
    field.options = [...(field.options ?? []), ''];
    fields = [...fields];
    onchange?.(fields);
  }

  function updateOption(fieldIndex: number, optionIndex: number, value: string) {
    const field = fields[fieldIndex];
    if (field.options) {
      field.options[optionIndex] = value;
      fields = [...fields];
      onchange?.(fields);
    }
  }

  function removeOption(fieldIndex: number, optionIndex: number) {
    const field = fields[fieldIndex];
    if (field.options) {
      field.options = field.options.filter((_, i) => i !== optionIndex);
      fields = [...fields];
      onchange?.(fields);
    }
  }
</script>

<div class="space-y-6">
  {#each fields as field, i}
    <div class="border rounded-lg p-4 space-y-4">
      <div class="flex justify-between items-center">
        <h4 class="font-semibold">Field {i + 1}</h4>
        <Button variant="destructive" size="sm" onclick={() => removeField(i)}>
          Remove
        </Button>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <Label for={`key-${i}`}>Key (unique identifier)</Label>
          <Input
            id={`key-${i}`}
            type="text"
            value={field.key}
            oninput={(e) => updateField(i, { key: e.currentTarget.value })}
            placeholder="e.g., read_status"
          />
        </div>

        <div>
          <Label for={`label-${i}`}>Label</Label>
          <Input
            id={`label-${i}`}
            type="text"
            value={field.label}
            oninput={(e) => updateField(i, { label: e.currentTarget.value })}
            placeholder="e.g., Read Status"
          />
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <Label for={`type-${i}`}>Field Type</Label>
          <select
            id={`type-${i}`}
            value={field.type}
            onchange={(e) => updateField(i, { type: e.currentTarget.value as FieldType })}
            class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
          >
            <option value="text">Text</option>
            <option value="number">Number</option>
            <option value="date">Date</option>
            <option value="checkbox">Checkbox</option>
            <option value="select">Select (dropdown)</option>
            <option value="multi_select">Multi-select</option>
          </select>
        </div>

        <div class="flex items-center space-x-2 pt-6">
          <Checkbox
            id={`required-${i}`}
            checked={field.required}
            onCheckedChange={(checked) => updateField(i, { required: checked as boolean })}
          />
          <Label for={`required-${i}`}>Required field</Label>
        </div>
      </div>

      <div>
        <Label for={`description-${i}`}>Description (optional)</Label>
        <Input
          id={`description-${i}`}
          type="text"
          value={field.description ?? ''}
          oninput={(e) => updateField(i, { description: e.currentTarget.value })}
          placeholder="Help text for users"
        />
      </div>

      {#if field.type === 'select' || field.type === 'multi_select'}
        <div>
          <Label>Options</Label>
          <div class="space-y-2 mt-2">
            {#each field.options ?? [] as option, optIndex}
              <div class="flex gap-2">
                <Input
                  type="text"
                  value={option}
                  oninput={(e) => updateOption(i, optIndex, e.currentTarget.value)}
                  placeholder="Option value"
                />
                <Button
                  variant="outline"
                  size="sm"
                  onclick={() => removeOption(i, optIndex)}
                >
                  Remove
                </Button>
              </div>
            {/each}
            <Button variant="outline" size="sm" onclick={() => addOption(i)}>
              + Add Option
            </Button>
          </div>
        </div>
      {/if}
    </div>
  {/each}

  <Button onclick={addField}>+ Add Field</Button>
</div>
