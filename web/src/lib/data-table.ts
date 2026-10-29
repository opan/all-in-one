import {
	columnFilteringFeature,
	columnVisibilityFeature,
	createFilteredRowModel,
	createPaginatedRowModel,
	globalFilteringFeature,
	rowPaginationFeature,
	tableFeatures,
	type ColumnDef,
	type Row,
	type RowData
} from '@tanstack/svelte-table';

// Features of the shared data table (src/components/data-table.svelte): a
// text filter across all columns, column visibility and client-side pages.
// TanStack Table v9 only includes the features a table registers.
export const dataTableFeatures = tableFeatures({
	columnFilteringFeature,
	globalFilteringFeature,
	filteredRowModel: createFilteredRowModel(),
	columnVisibilityFeature,
	rowPaginationFeature,
	paginatedRowModel: createPaginatedRowModel()
});

export type DataTableFeatures = typeof dataTableFeatures;
export type DataTableColumn<TData extends RowData> = ColumnDef<DataTableFeatures, TData>;
export type DataTableRow<TData extends RowData> = Row<DataTableFeatures, TData>;
