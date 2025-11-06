<script lang="ts" generics="TData">
  import { createSvelteTable } from "$lib/components/ui/data-table/data-table.svelte";
  import { FlexRender } from "$lib/components/ui/data-table";
  import * as Table from "$lib/components/ui/table/index";
  import { Button } from "$lib/components/ui/button/index";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index";
  import { Input } from "$lib/components/ui/input/index";
  import type { Snippet } from "svelte";
  import { 
    getCoreRowModel, 
    getFilteredRowModel,
    getPaginationRowModel,
    type ColumnDef,
    type Row
  } from "@tanstack/table-core";
  import { ChevronDown } from "@lucide/svelte";

  interface Props {
    data: TData[];
    columns: ColumnDef<TData>[];
    filterPlaceholder?: string;
    showFilter?: boolean;
    showColumnVisibility?: boolean;
    showPagination?: boolean;
    actions?: {
      label: string;
      variant?: "default" | "outline" | "destructive" | "secondary" | "ghost" | "link";
      onclick: () => void;
    }[];
    actionsColumn?: Snippet<[{ row: Row<TData> }]>;
  }

  let {
    data,
    columns,
    filterPlaceholder = "Filter...",
    showFilter = true,
    showColumnVisibility = true,
    showPagination = true,
    actions = [],
    actionsColumn
  }: Props = $props();

  let filtering = $state('');
  let columnVisibility = $state<Record<string, boolean>>({});

  const table = createSvelteTable({
    get data() {
      return data;
    },
    columns,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    onColumnVisibilityChange: (updater) => {
      if (typeof updater === 'function') {
        columnVisibility = updater(columnVisibility);
      } else {
        columnVisibility = updater;
      }
    },
    onGlobalFilterChange: (updater) => {
      if (typeof updater === 'function') {
        filtering = updater(filtering);
      } else {
        filtering = updater;
      }
    },
    state: {
      get globalFilter() {
        return filtering;
      },
      get columnVisibility() {
        return columnVisibility;
      },
    },
  });
</script>

<div class="space-y-4">
  {#if showFilter || showColumnVisibility || actions.length > 0}
    <div class="flex items-center justify-between gap-4">
      {#if showFilter}
        <Input
          placeholder={filterPlaceholder}
          value={filtering}
          oninput={(e) => filtering = e.currentTarget.value}
          class="max-w-sm"
        />
      {/if}
      
      <div class="flex items-center gap-2 ml-auto">
        {#if showColumnVisibility}
          <DropdownMenu.Root>
            <DropdownMenu.Trigger>
              <Button variant="outline">
                Columns <ChevronDown class="ml-2 h-4 w-4" />
              </Button>
            </DropdownMenu.Trigger>
            <DropdownMenu.Content align="end">
              {#each table.getAllLeafColumns() as column}
                {#if column.getCanHide()}
                  <DropdownMenu.CheckboxItem
                    checked={column.getIsVisible()}
                    onCheckedChange={(value) => column.toggleVisibility(!!value)}
                  >
                    {column.id}
                  </DropdownMenu.CheckboxItem>
                {/if}
              {/each}
            </DropdownMenu.Content>
          </DropdownMenu.Root>
        {/if}
        
        {#each actions as action}
          <Button 
            variant={action.variant || "default"}
            onclick={action.onclick}
          >
            {action.label}
          </Button>
        {/each}
      </div>
    </div>
  {/if}

  <div class="rounded-md border">
    <Table.Root>
      <Table.Header>
        {#each table.getHeaderGroups() as headerGroup}
          <Table.Row>
            {#each headerGroup.headers as header}
              <Table.Head>
                {#if !header.isPlaceholder}
                  <FlexRender
                    content={header.column.columnDef.header}
                    context={header.getContext()}
                  />
                {/if}
              </Table.Head>
            {/each}
          </Table.Row>
        {/each}
      </Table.Header>
      <Table.Body>
        {#if table.getRowModel().rows?.length}
          {#each table.getRowModel().rows as row}
            <Table.Row>
              {#each row.getVisibleCells() as cell}
                <Table.Cell>
                  {#if cell.column.id === 'actions' && actionsColumn}
                    {@render actionsColumn({ row })}
                  {:else}
                    <FlexRender
                      content={cell.column.columnDef.cell}
                      context={cell.getContext()}
                    />
                  {/if}
                </Table.Cell>
              {/each}
            </Table.Row>
          {/each}
        {:else}
          <Table.Row>
            <Table.Cell colspan={columns.length} class="h-24 text-center">
              No results.
            </Table.Cell>
          </Table.Row>
        {/if}
      </Table.Body>
    </Table.Root>
  </div>

  {#if showPagination}
    <div class="flex items-center justify-between space-x-2">
      <div class="text-sm text-muted-foreground">
        {table.getFilteredRowModel().rows.length} row(s) total.
      </div>
      <div class="flex items-center space-x-2">
        <Button
          variant="outline"
          size="sm"
          onclick={() => table.previousPage()}
          disabled={!table.getCanPreviousPage()}
        >
          Previous
        </Button>
        <Button
          variant="outline"
          size="sm"
          onclick={() => table.nextPage()}
          disabled={!table.getCanNextPage()}
        >
          Next
        </Button>
      </div>
    </div>
  {/if}
</div>
