<script lang="ts">
  export let data: { listings: Item[] };
  let listings: Item[] = data.listings;
  
  interface Item {
    id: number;
    title: string;
    description: string;
    created_at: string;
    updated_at: string;
  }
  
  // Form state
  let showAddForm = false;
  let editingItem: number | null = null;
  let formData = {
    title: '',
    description: ''
  };
  
  // Loading and error states
  let loading = false;
  let error = '';
  
  // Add new item
  async function addItem() {
    if (!formData.title.trim() || !formData.description.trim()) {
      error = 'Title and description are required';
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      const response = await fetch('/api/v1/items', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          title: formData.title.trim(),
          description: formData.description.trim()
        }),
      });
      
      if (!response.ok) {
        throw new Error('Failed to create item');
      }
      
      const newItem = await response.json();
      listings = [...listings, newItem.data];
      
      // Reset form
      formData = { title: '', description: '' };
      showAddForm = false;
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }
  
  // Edit item
  function startEdit(item: Item) {
    editingItem = item.id;
    formData = {
      title: item.title,
      description: item.description
    };
  }
  
  function cancelEdit() {
    editingItem = null;
    formData = { title: '', description: '' };
  }
  
  async function saveEdit(id: number) {
    if (!formData.title.trim() || !formData.description.trim()) {
      error = 'Title and description are required';
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      const response = await fetch(`/api/v1/items/${id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          title: formData.title.trim(),
          description: formData.description.trim()
        }),
      });
      
      if (!response.ok) {
        throw new Error('Failed to update item');
      }
      
      const updatedItem = await response.json();
      listings = listings.map((item: Item) => 
        item.id === id ? updatedItem.data : item
      );
      
      editingItem = null;
      formData = { title: '', description: '' };
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }
  
  // Delete item
  async function deleteItem(id: number) {
    if (!confirm('Are you sure you want to delete this item?')) {
      return;
    }
    
    loading = true;
    error = '';
    
    try {
      const response = await fetch(`/api/v1/items/${id}`, {
        method: 'DELETE',
      });
      
      if (!response.ok) {
        throw new Error('Failed to delete item');
      }
      
      listings = listings.filter((item: Item) => item.id !== id);
    } catch (err) {
      error = err instanceof Error ? err.message : 'An unexpected error occurred';
    } finally {
      loading = false;
    }
  }
  
  function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
  }
</script>

<style>
  .container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 2rem;
  }
  
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 2rem;
  }
  
  .btn {
    padding: 0.5rem 1rem;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-size: 0.9rem;
  }
  
  .btn-primary {
    background-color: #007bff;
    color: white;
  }
  
  .btn-primary:hover {
    background-color: #0056b3;
  }
  
  .btn-secondary {
    background-color: #6c757d;
    color: white;
  }
  
  .btn-secondary:hover {
    background-color: #545b62;
  }
  
  .btn-danger {
    background-color: #dc3545;
    color: white;
  }
  
  .btn-danger:hover {
    background-color: #c82333;
  }
  
  .btn-small {
    padding: 0.25rem 0.5rem;
    font-size: 0.8rem;
    margin: 0 0.25rem;
  }
  
  .form-container {
    background-color: #f8f9fa;
    padding: 1.5rem;
    border-radius: 8px;
    margin-bottom: 2rem;
  }
  
  .form-group {
    margin-bottom: 1rem;
  }
  
  .form-group label {
    display: block;
    margin-bottom: 0.5rem;
    font-weight: bold;
  }
  
  .form-group input,
  .form-group textarea {
    width: 100%;
    padding: 0.5rem;
    border: 1px solid #ddd;
    border-radius: 4px;
    font-size: 1rem;
  }
  
  .form-group textarea {
    resize: vertical;
    min-height: 80px;
  }
  
  .form-actions {
    display: flex;
    gap: 1rem;
  }
  
  .table-container {
    overflow-x: auto;
  }
  
  table {
    width: 100%;
    border-collapse: collapse;
    background-color: white;
    border-radius: 8px;
    overflow: hidden;
    box-shadow: 0 2px 4px rgba(0,0,0,0.1);
  }
  
  th, td {
    padding: 1rem;
    text-align: left;
    border-bottom: 1px solid #ddd;
  }
  
  th {
    background-color: #f8f9fa;
    font-weight: bold;
  }
  
  tr:hover {
    background-color: #f8f9fa;
  }
  
  .actions {
    white-space: nowrap;
  }
  
  .error {
    background-color: #f8d7da;
    color: #721c24;
    padding: 0.75rem;
    border-radius: 4px;
    margin-bottom: 1rem;
  }
  
  .loading {
    opacity: 0.6;
    pointer-events: none;
  }
  
  .edit-form {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }
  
  .edit-form input,
  .edit-form textarea {
    padding: 0.25rem;
    border: 1px solid #ddd;
    border-radius: 4px;
  }
  
  .edit-actions {
    display: flex;
    gap: 0.25rem;
  }
</style>

<div class="container" class:loading>
  <div class="header">
    <h1>Listing Items</h1>
    <button 
      class="btn btn-primary" 
      on:click={() => showAddForm = !showAddForm}
      disabled={loading}
    >
      {showAddForm ? 'Cancel' : 'Add New Item'}
    </button>
  </div>
  
  {#if error}
    <div class="error">{error}</div>
  {/if}
  
  {#if showAddForm}
    <div class="form-container">
      <h3>Add New Item</h3>
      <form on:submit|preventDefault={addItem}>
        <div class="form-group">
          <label for="title">Title</label>
          <input 
            id="title"
            type="text" 
            bind:value={formData.title}
            disabled={loading}
            required
          />
        </div>
        
        <div class="form-group">
          <label for="description">Description</label>
          <textarea 
            id="description"
            bind:value={formData.description}
            disabled={loading}
            required
          ></textarea>
        </div>
        
        <div class="form-actions">
          <button type="submit" class="btn btn-primary" disabled={loading}>
            {loading ? 'Adding...' : 'Add Item'}
          </button>
          <button 
            type="button" 
            class="btn btn-secondary" 
            on:click={() => showAddForm = false}
            disabled={loading}
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  {/if}
  
  {#if listings.length > 0}
    <div class="table-container">
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Title</th>
            <th>Description</th>
            <th>Created</th>
            <th>Updated</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each listings as item}
            <tr>
              <td>{item.id}</td>
              <td>
                {#if editingItem === item.id}
                  <div class="edit-form">
                    <input 
                      type="text" 
                      bind:value={formData.title}
                      disabled={loading}
                    />
                  </div>
                {:else}
                  {item.title}
                {/if}
              </td>
              <td>
                {#if editingItem === item.id}
                  <div class="edit-form">
                    <textarea 
                      bind:value={formData.description}
                      disabled={loading}
                    ></textarea>
                  </div>
                {:else}
                  {item.description}
                {/if}
              </td>
              <td>{formatDate(item.created_at)}</td>
              <td>{formatDate(item.updated_at)}</td>
              <td class="actions">
                {#if editingItem === item.id}
                  <div class="edit-actions">
                    <button 
                      class="btn btn-primary btn-small"
                      on:click={() => saveEdit(item.id)}
                      disabled={loading}
                    >
                      Save
                    </button>
                    <button 
                      class="btn btn-secondary btn-small"
                      on:click={cancelEdit}
                      disabled={loading}
                    >
                      Cancel
                    </button>
                  </div>
                {:else}
                  <button 
                    class="btn btn-primary btn-small"
                    on:click={() => startEdit(item)}
                    disabled={loading}
                  >
                    Edit
                  </button>
                  <button 
                    class="btn btn-danger btn-small"
                    on:click={() => deleteItem(item.id)}
                    disabled={loading}
                  >
                    Delete
                  </button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {:else}
    <p>No listings found. <button class="btn btn-primary" on:click={() => showAddForm = true}>Add the first item</button></p>
  {/if}
</div>
