// JSONForms.io compliant schema types

// Type-safe alternative to 'any' for JSON values
export type JSONValue =
	| string
	| number
	| boolean
	| null
	| JSONValue[]
	| { [key: string]: JSONValue };

export interface JSONSchema {
	type: "object";
	properties: Record<
		string,
		{
			type: "string" | "number" | "integer" | "boolean" | "array";
			title?: string;
			description?: string;
			enum?: string[];
			format?: string;
			default?: JSONValue;
			items?: {
				type: string;
				enum?: string[];
			};
		}
	>;
	required?: string[];
}

export interface UISchemaElement {
	type: "Control" | "VerticalLayout" | "HorizontalLayout" | "Group";
	scope?: string;
	label?: string | boolean;
	options?: {
		format?: string;
		readonly?: boolean;
		[key: string]: JSONValue | undefined;
	};
	elements?: UISchemaElement[];
}

export interface UISchema extends UISchemaElement {
	type: "VerticalLayout" | "HorizontalLayout" | "Group";
	elements: UISchemaElement[];
}

export interface FormSchema {
	schema: JSONSchema;
	uischema: UISchema;
}

export interface Topic {
	id: number;
	name: string;
	description: string;
	form_schema?: FormSchema;
	created_at: string;
	updated_at: string;
}

export interface Item {
	id: number;
	title: string;
	description: string;
	topic_id: number;
	form_schema_values?: Record<string, JSONValue>;
	created_at: string;
	updated_at: string;
}
