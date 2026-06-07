import { writable } from "svelte/store";
import { browser } from "$app/environment";

export type PaletteKey = "default" | "ocean" | "slate" | "forest";

export interface PaletteMeta {
	key: PaletteKey;
	label: string;
	swatches: [string, string, string, string];
}

export const PALETTES: PaletteMeta[] = [
	{
		key: "default",
		label: "Default",
		swatches: ["#FAFAFA", "#A1A1AA", "#52525B", "#18181B"],
	},
	{
		key: "ocean",
		label: "Ocean",
		swatches: ["#2C5EAD", "#1591DC", "#4BB8FA", "#C4E2F5"],
	},
	{
		key: "slate",
		label: "Slate",
		swatches: ["#F5F5F5", "#76ABAE", "#303841", "#FF5722"],
	},
	{
		key: "forest",
		label: "Forest",
		swatches: ["#5B7E3C", "#FFD65A", "#FF9D23", "#EA5252"],
	},
];

const STORAGE_KEY = "aio-palette";
const DEFAULT_PALETTE: PaletteKey = "default";

export const palette = writable<PaletteKey>(DEFAULT_PALETTE);

function isPaletteKey(value: string | null): value is PaletteKey {
	return value !== null && PALETTES.some((p) => p.key === value);
}

export function setPalette(key: PaletteKey): void {
	palette.set(key);
	if (!browser) return;
	localStorage.setItem(STORAGE_KEY, key);
	document.documentElement.dataset.palette = key;
	// TODO: sync to backend (PATCH /api/v1/users/preferences) once the endpoint exists.
}

export function initPalette(): void {
	if (!browser) return;
	const stored = localStorage.getItem(STORAGE_KEY);
	const key: PaletteKey = isPaletteKey(stored) ? stored : DEFAULT_PALETTE;
	palette.set(key);
	document.documentElement.dataset.palette = key;
}
