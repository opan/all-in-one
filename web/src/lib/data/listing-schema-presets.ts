import type { FormSchema } from "$lib/types/json-forms";

// A curated JSONForms schema a user can apply as a starting point when
// authoring a topic's form_schema. Reuses the existing FormSchema type so every
// preset is type-checked at build time.
export interface TopicSchemaPreset {
	id: string;
	name: string;
	description: string;
	icon?: string;
	formSchema: FormSchema;
}

// Build a VerticalLayout uischema with one Control per property key, preserving order.
function verticalLayout(keys: string[]): FormSchema["uischema"] {
	return {
		type: "VerticalLayout",
		elements: keys.map((key) => ({
			type: "Control",
			scope: `#/properties/${key}`
		}))
	};
}

const productListing: FormSchema = {
	schema: {
		type: "object",
		properties: {
			title: { type: "string", title: "Title", description: "Product name" },
			price: { type: "number", title: "Price", description: "Price in USD" },
			quantity: { type: "integer", title: "Quantity", default: 1 },
			category: {
				type: "string",
				title: "Category",
				enum: ["electronics", "furniture", "clothing", "other"]
			},
			in_stock: { type: "boolean", title: "In Stock", default: true },
			description: { type: "string", title: "Description" }
		},
		required: ["title"]
	},
	uischema: verticalLayout(["title", "price", "quantity", "category", "in_stock", "description"])
};

const contact: FormSchema = {
	schema: {
		type: "object",
		properties: {
			full_name: { type: "string", title: "Full Name" },
			email: { type: "string", title: "Email", format: "email" },
			phone: { type: "string", title: "Phone" },
			company: { type: "string", title: "Company" },
			notes: { type: "string", title: "Notes" }
		},
		required: ["full_name"]
	},
	uischema: verticalLayout(["full_name", "email", "phone", "company", "notes"])
};

const task: FormSchema = {
	schema: {
		type: "object",
		properties: {
			title: { type: "string", title: "Title" },
			status: {
				type: "string",
				title: "Status",
				enum: ["todo", "in-progress", "done"],
				default: "todo"
			},
			priority: {
				type: "string",
				title: "Priority",
				enum: ["low", "medium", "high"],
				default: "medium"
			},
			due_date: { type: "string", title: "Due Date", format: "date" },
			done: { type: "boolean", title: "Done", default: false }
		},
		required: ["title"]
	},
	uischema: verticalLayout(["title", "status", "priority", "due_date", "done"])
};

const event: FormSchema = {
	schema: {
		type: "object",
		properties: {
			name: { type: "string", title: "Name" },
			start: { type: "string", title: "Start", format: "date-time" },
			location: { type: "string", title: "Location" },
			url: { type: "string", title: "URL", format: "uri" },
			notes: { type: "string", title: "Notes" }
		},
		required: ["name"]
	},
	uischema: verticalLayout(["name", "start", "location", "url", "notes"])
};

export const topicSchemaPresets: TopicSchemaPreset[] = [
	{
		id: "product-listing",
		name: "Product listing",
		description: "Sell or catalog items with price, quantity, and stock.",
		icon: "🛍️",
		formSchema: productListing
	},
	{
		id: "contact",
		name: "Contact",
		description: "Keep people's details — email, phone, and company.",
		icon: "👤",
		formSchema: contact
	},
	{
		id: "task",
		name: "Task / To-do",
		description: "Track work with status, priority, and due date.",
		icon: "✅",
		formSchema: task
	},
	{
		id: "event",
		name: "Event",
		description: "Plan events with a start time, location, and link.",
		icon: "📅",
		formSchema: event
	}
];
