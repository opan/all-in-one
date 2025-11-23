<script lang="ts">
  import { Input } from '$lib/components/ui/input';
  import { Label } from '$lib/components/ui/label';
  import { Checkbox } from '$lib/components/ui/checkbox';
  import type { CustomField, ItemCustomData } from '$lib/types/topic';

  interface Props {
    fields: CustomField[];
    data?: ItemCustomData;
    onchange?: (data: ItemCustomData) => void;
  }

  let { fields, data = $bindable({}), onchange }: Props = $props();

  function handleChange(key: string, value: any) {
    data[key] = value;
    onchange?.(data);
  }

  // function getSelectValue(key: string, defaultValue?: any): string | undefined {
  //   return data[key] ?? defaultValue;
  // }
</script>

<div class="space-y-4">
  {#each fields as field}
    <div class="space-y-2">
      <Label for={field.key}>
        {field.label}
        {#if field.required}
          <span class="text-red-500">*</span>
        {/if}
      </Label>
      
      {#if field.description}
        <p class="text-sm text-muted-foreground">{field.description}</p>
      {/if}

      {#if field.type === 'text'}
        <Input
          id={field.key}
          type="text"
          value={data[field.key] ?? field.default_value ?? ''}
          oninput={(e) => handleChange(field.key, e.currentTarget.value)}
          required={field.required}
        />

      {:else if field.type === 'number'}
        <Input
          id={field.key}
          type="number"
          value={data[field.key] ?? field.default_value ?? ''}
          oninput={(e) => handleChange(field.key, Number(e.currentTarget.value))}
          required={field.required}
        />

      {:else if field.type === 'date'}
        <Input
          id={field.key}
          type="date"
          value={data[field.key] ?? field.default_value ?? ''}
          oninput={(e) => handleChange(field.key, e.currentTarget.value)}
          required={field.required}
        />

      {:else if field.type === 'checkbox'}
        {#if data[field.key] ?? field.default_value ?? false}
          <Checkbox
            id={field.key}
            checked
            onCheckedChange={(checked: boolean) => handleChange(field.key, checked)}
          />
        {:else}
          <Checkbox
            id={field.key}
            onCheckedChange={(checked: boolean) => handleChange(field.key, checked)}
          />
        {/if}

      {:else if field.type === 'select'}
        <select
          id={field.key}
          value={data[field.key] ?? field.default_value ?? ''}
          onchange={(e) => handleChange(field.key, e.currentTarget.value)}
          required={field.required}
          class="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
        >
          <option value="">Select an option</option>
          {#each field.options ?? [] as option}
            <option value={option}>{option}</option>
          {/each}
        </select>

      {:else if field.type === 'multi_select'}
        <div class="space-y-2">
          {#each field.options ?? [] as option}
            <div class="flex items-center space-x-2">
              {#if ((data[field.key] as string[]) ?? []).includes(option)}
                <Checkbox
                  id={`${field.key}-${option}`}
                  checked
                  onCheckedChange={(checked: boolean) => {
                    const current = (data[field.key] as string[]) ?? [];
                    if (checked) {
                      handleChange(field.key, [...current, option]);
                    } else {
                      handleChange(field.key, current.filter(v => v !== option));
                    }
                  }}
                />
              {:else}
                <Checkbox
                  id={`${field.key}-${option}`}
                  onCheckedChange={(checked: boolean) => {
                    const current = (data[field.key] as string[]) ?? [];
                    if (checked) {
                      handleChange(field.key, [...current, option]);
                    } else {
                      handleChange(field.key, current.filter(v => v !== option));
                    }
                  }}
                />
              {/if}
              <Label for={`${field.key}-${option}`}>{option}</Label>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/each}
</div>
