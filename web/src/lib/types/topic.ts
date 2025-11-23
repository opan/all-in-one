export type FieldType = 
  | 'text'
  | 'number'
  | 'select'
  | 'multi_select'
  | 'checkbox'
  | 'date';

export interface CustomField {
  key: string;
  label: string;
  type: FieldType;
  required: boolean;
  options?: string[];
  default_value?: any;
  description?: string;
}

export interface FormSchema {
  fields: CustomField[];
}

export interface Topic {
  id: number;
  name: string;
  description: string;
  form_schema: FormSchema;
  created_at: string;
  updated_at: string;
}

// For item data that conforms to a topic's schema
export interface ItemCustomData {
  [key: string]: string | number | boolean | string[];
}
