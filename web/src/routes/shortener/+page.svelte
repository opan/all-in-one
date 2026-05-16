<script lang="ts">
  import { Button } from '$lib/components/ui/button/index';
  import { Input } from '$lib/components/ui/input/index';
  import { Label } from '$lib/components/ui/label/index';
  import { Switch } from '$lib/components/ui/switch/index';
  import * as Dialog from '$lib/components/ui/dialog/index';
  import * as AlertDialog from '$lib/components/ui/alert-dialog/index';
  import * as Table from '$lib/components/ui/table/index';
  import {
    Link2,
    Copy,
    Check,
    Trash2,
    Plus,
    ExternalLink,
    Loader2
  } from '@lucide/svelte/icons';
  import {
    type ShortLink,
    createShortLink,
    updateShortLink,
    deleteShortLink,
    listShortLinks,
    shortURL,
  } from '$lib/shortener-api';

  interface Props {
    data: { links: ShortLink[]; total: number };
  }

  let { data }: Props = $props();

  let links = $state<ShortLink[]>(data.links ?? []);
  let total = $state(data.total ?? 0);

  // Create dialog
  let createOpen = $state(false);
  let createURL = $state('');
  let createExpiry = $state('');
  let createError = $state('');
  let creating = $state(false);

  // Delete dialog
  let deleteOpen = $state(false);
  let linkToDelete = $state<ShortLink | null>(null);
  let deleting = $state(false);

  // Copy feedback: code → true for 2s
  let copied = $state<Record<string, boolean>>({});

  // Per-row toggle loading
  let toggling = $state<Record<string, boolean>>({});

  // General error
  let error = $state('');

  async function reload() {
    try {
      const page = await listShortLinks(1, 50);
      links = page.links;
      total = page.total;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to reload links';
    }
  }

  async function handleCreate() {
    createError = '';
    if (!createURL.trim()) {
      createError = 'URL is required';
      return;
    }
    creating = true;
    try {
      const payload: { target_url: string; expires_at?: string } = {
        target_url: createURL.trim()
      };
      if (createExpiry) payload.expires_at = new Date(createExpiry).toISOString();
      await createShortLink(payload);
      createOpen = false;
      createURL = '';
      createExpiry = '';
      await reload();
    } catch (e) {
      createError = e instanceof Error ? e.message : 'Failed to create short link';
    } finally {
      creating = false;
    }
  }

  async function handleToggleActive(link: ShortLink) {
    toggling = { ...toggling, [link.code]: true };
    try {
      const updated = await updateShortLink(link.code, { is_active: !link.is_active });
      links = links.map(l => l.code === link.code ? updated : l);
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to update link';
    } finally {
      toggling = { ...toggling, [link.code]: false };
    }
  }

  function confirmDelete(link: ShortLink) {
    linkToDelete = link;
    deleteOpen = true;
  }

  async function handleDelete() {
    if (!linkToDelete) return;
    deleting = true;
    try {
      await deleteShortLink(linkToDelete.code);
      links = links.filter(l => l.code !== linkToDelete!.code);
      total = Math.max(0, total - 1);
      deleteOpen = false;
      linkToDelete = null;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Failed to delete link';
    } finally {
      deleting = false;
    }
  }

  async function copyToClipboard(code: string) {
    try {
      await navigator.clipboard.writeText(shortURL(code));
      copied = { ...copied, [code]: true };
      setTimeout(() => { copied = { ...copied, [code]: false }; }, 2000);
    } catch {
      // clipboard not available (non-https)
    }
  }

  function formatDate(iso: string | null | undefined): string {
    if (!iso) return '—';
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric', month: 'short', day: 'numeric'
    });
  }

  function truncate(str: string, n: number): string {
    return str.length > n ? str.slice(0, n) + '…' : str;
  }
</script>

<div class="container mx-auto p-6">
  <!-- Header -->
  <div class="mb-6 flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-bold tracking-tight">URL Shortener</h1>
      <p class="text-muted-foreground">{total} {total === 1 ? 'link' : 'links'}</p>
    </div>
    <Button onclick={() => { createOpen = true; createError = ''; }}>
      <Plus class="mr-1 size-4" />
      Shorten URL
    </Button>
  </div>

  {#if error}
    <div class="mb-4 rounded-md border border-destructive/40 bg-destructive/10 px-4 py-2 text-sm text-destructive">
      {error}
      <button class="ml-2 underline" onclick={() => error = ''}>dismiss</button>
    </div>
  {/if}

  <!-- Links table -->
  {#if links.length === 0}
    <div class="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed py-16 text-muted-foreground">
      <Link2 class="size-10 opacity-40" />
      <p class="text-sm">No short links yet. Click <strong>Shorten URL</strong> to create one.</p>
    </div>
  {:else}
    <div class="rounded-md border">
      <Table.Root>
        <Table.Header>
          <Table.Row>
            <Table.Head class="w-32">Short code</Table.Head>
            <Table.Head>Target URL</Table.Head>
            <Table.Head class="w-20 text-right">Clicks</Table.Head>
            <Table.Head class="w-20">Status</Table.Head>
            <Table.Head class="w-32">Expires</Table.Head>
            <Table.Head class="w-24 text-right">Actions</Table.Head>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {#each links as link (link.code)}
            <Table.Row class={link.is_active ? '' : 'opacity-50'}>
              <!-- Short code + copy -->
              <Table.Cell class="font-mono text-sm">
                <div class="flex items-center gap-1.5">
                  <span>{link.code}</span>
                  <Button
                    variant="ghost"
                    size="icon"
                    class="size-6 text-muted-foreground hover:text-foreground"
                    onclick={() => copyToClipboard(link.code)}
                    title="Copy short URL"
                  >
                    {#if copied[link.code]}
                      <Check class="size-3.5 text-green-500" />
                    {:else}
                      <Copy class="size-3.5" />
                    {/if}
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon"
                    class="size-6 text-muted-foreground hover:text-foreground"
                    title="Open short link"
                  >
                    {#snippet child({ props })}
                      <a href={shortURL(link.code)} target="_blank" rel="noopener noreferrer" {...props}>
                        <ExternalLink class="size-3.5" />
                      </a>
                    {/snippet}
                  </Button>
                </div>
              </Table.Cell>

              <!-- Target URL -->
              <Table.Cell class="max-w-xs">
                <a
                  href={link.target_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="text-sm text-muted-foreground hover:text-foreground hover:underline"
                  title={link.target_url}
                >
                  {truncate(link.target_url, 60)}
                </a>
              </Table.Cell>

              <!-- Clicks -->
              <Table.Cell class="text-right text-sm tabular-nums">
                {link.click_count}
              </Table.Cell>

              <!-- Status -->
              <Table.Cell>
                <span class="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium
                  {link.is_active
                    ? 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
                    : 'bg-muted text-muted-foreground'}">
                  {link.is_active ? 'Active' : 'Inactive'}
                </span>
              </Table.Cell>

              <!-- Expires -->
              <Table.Cell class="text-sm text-muted-foreground">
                {formatDate(link.expires_at)}
              </Table.Cell>

              <!-- Actions -->
              <Table.Cell class="text-right">
                <div class="flex items-center justify-end gap-2">
                  {#if toggling[link.code]}
                    <Loader2 class="size-4 animate-spin text-muted-foreground" />
                  {:else}
                    <Switch
                      checked={link.is_active}
                      disabled={toggling[link.code]}
                      onCheckedChange={() => handleToggleActive(link)}
                    />
                  {/if}
                  <Button
                    variant="ghost"
                    size="icon"
                    class="text-muted-foreground hover:bg-destructive/10 hover:text-destructive"
                    onclick={() => confirmDelete(link)}
                    title="Delete link"
                  >
                    <Trash2 class="size-4" />
                  </Button>
                </div>
              </Table.Cell>
            </Table.Row>
          {/each}
        </Table.Body>
      </Table.Root>
    </div>
  {/if}
</div>

<!-- Create dialog -->
<Dialog.Root bind:open={createOpen}>
  <Dialog.Content class="sm:max-w-md">
    <Dialog.Header>
      <Dialog.Title>Shorten a URL</Dialog.Title>
      <Dialog.Description>
        Paste a long URL to generate a short link.
      </Dialog.Description>
    </Dialog.Header>

    <div class="flex flex-col gap-4 py-2">
      <div class="flex flex-col gap-1.5">
        <Label for="target-url">Target URL <span class="text-destructive">*</span></Label>
        <Input
          id="target-url"
          type="url"
          placeholder="https://example.com/very/long/path"
          bind:value={createURL}
          onkeydown={(e) => e.key === 'Enter' && handleCreate()}
        />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="expiry">Expires at <span class="text-muted-foreground text-xs">(optional)</span></Label>
        <Input
          id="expiry"
          type="datetime-local"
          bind:value={createExpiry}
        />
      </div>
      {#if createError}
        <p class="text-sm text-destructive">{createError}</p>
      {/if}
    </div>

    <Dialog.Footer>
      <Button variant="outline" onclick={() => createOpen = false}>Cancel</Button>
      <Button onclick={handleCreate} disabled={creating}>
        {#if creating}
          <Loader2 class="mr-2 size-4 animate-spin" />
        {/if}
        Create
      </Button>
    </Dialog.Footer>
  </Dialog.Content>
</Dialog.Root>

<!-- Delete confirmation -->
<AlertDialog.Root bind:open={deleteOpen}>
  <AlertDialog.Content>
    <AlertDialog.Header>
      <AlertDialog.Title>Delete short link?</AlertDialog.Title>
      <AlertDialog.Description>
        The short link <strong class="font-mono">{linkToDelete?.code}</strong> will be permanently
        deleted. Anyone following it will get a 404.
      </AlertDialog.Description>
    </AlertDialog.Header>
    <AlertDialog.Footer>
      <AlertDialog.Cancel>Cancel</AlertDialog.Cancel>
      <AlertDialog.Action
        onclick={handleDelete}
        disabled={deleting}
        class="bg-destructive text-destructive-foreground hover:bg-destructive/90"
      >
        {#if deleting}
          <Loader2 class="mr-2 size-4 animate-spin" />
        {/if}
        Delete
      </AlertDialog.Action>
    </AlertDialog.Footer>
  </AlertDialog.Content>
</AlertDialog.Root>
