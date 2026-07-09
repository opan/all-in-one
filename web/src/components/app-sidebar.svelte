<script lang="ts">
  import type { Component } from 'svelte';
  import * as Sidebar from "$lib/components/ui/sidebar/index";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index";
  import {
    SquareTerminal,
    Settings,
    ChevronDown,
    SquareUser,
    Table,
    LogOut,
    MessageSquare,
    Link2,
    Users,
    ShieldCheck,
  } from "@lucide/svelte/icons";
  import type { IconProps } from '@lucide/svelte';
	import TableBody from '$lib/components/ui/table/table-body.svelte';
  import { apiDelete, isLoggedIn } from '$lib/api';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { onMount } from 'svelte';
  import { useSidebar } from '$lib/components/ui/sidebar/context.svelte.js';
  import { auth, loadAuth, hasFeature } from '$lib/stores/auth';

  interface Props {
    currentPath?: string;
  }

  type NavItem = {
    title: string;
    url?: string;
    icon: Component<IconProps, {}, "">;
    isExpandable: boolean;
    feature?: string;
    subitems?: Array<{
      title: string;
      url: string;
      icon: Component<IconProps, {}, "">;
    }>;
  };

  let { currentPath = "/" }: Props = $props();
  let playgroundOpen = $state(true);

  const sidebar = useSidebar();

  $effect(() => {
    page.url.pathname;
    sidebar.setOpenMobile(false);
  });
  let userLoggedIn = $state(false);
  let isSigningOut = $state(false);

  onMount(async () => {
    userLoggedIn = await isLoggedIn();
    if (userLoggedIn) {
      await loadAuth();
    }
  });

  async function handleSignOut() {
    try {
      isSigningOut = true;
      const response = await apiDelete('/api/v1/sessions');
      
      if (response.ok) {
        // Redirect to login page after successful sign out
        await goto('/login', { replaceState: true });
      } else {
        console.error('Sign out failed');
        // Still redirect to login even if the API call fails
        await goto('/login', { replaceState: true });
      }
    } catch (error) {
      console.error('Sign out error:', error);
      // Still redirect to login even if there's an error
      await goto('/login', { replaceState: true });
    } finally {
      isSigningOut = false;
    }
  }

  const generalItems: NavItem[] = [
    {
      title: "Listings",
      url: "/listing/topics",
      icon: Table,
      isExpandable: false,
      feature: "listing",
    },
    {
      title: "Chats",
      url: "/chat",
      icon: MessageSquare,
      isExpandable: false,
      feature: "chat",
    },
    {
      title: "Shortener",
      url: "/shortener",
      icon: Link2,
      isExpandable: false,
      feature: "shortener",
    },
  ];

  // Cosmetic only — the backend (RequireFeature middleware) is the actual
  // enforcement point regardless of what's rendered here. Before auth loads
  // (userLoggedIn false, $auth null), show nothing rather than everything.
  let visibleGeneralItems = $derived(
    generalItems.filter((item) => !item.feature || hasFeature($auth, item.feature))
  );

  const adminItems: NavItem[] = [
    {
      title: "Users",
      url: "/admin/users",
      icon: Users,
      isExpandable: false,
    },
    {
      title: "Access",
      url: "/admin/access",
      icon: ShieldCheck,
      isExpandable: false,
    },
    {
      title: "Shortener",
      url: "/admin/shortener",
      icon: Link2,
      isExpandable: false,
    },
  ];

  // The whole Admin section only renders for admins. The backend RequireAdmin
  // middleware is the real gate; this is cosmetic (mirrors visibleGeneralItems).
  let showAdmin = $derived($auth?.is_admin === true);

  const settingItems: NavItem[] = [
    {
      title: "Settings",
      url: "/settings",
      icon: Settings,
      isExpandable: false,
    },
  ];
</script>

<Sidebar.Root collapsible="icon">
  <Sidebar.Header>
    <Sidebar.Menu>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton size="lg" tooltipContent="All-in-one platform">
          {#snippet child({ props })}
            <a href="/" {...props}>
              <div class="flex aspect-square size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <SquareTerminal class="size-4" />
              </div>
              <div class="grid flex-1 text-left text-sm leading-tight">
                <span class="truncate font-semibold">All-in-one</span>
                <span class="truncate text-xs text-muted-foreground">Platform</span>
              </div>
            </a>
          {/snippet}
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
    </Sidebar.Menu>
  </Sidebar.Header>
  
  <Sidebar.Content>
    <Sidebar.Group>
      <Sidebar.GroupLabel>General</Sidebar.GroupLabel>
      <Sidebar.GroupContent>
        <Sidebar.Menu>
          {#each visibleGeneralItems as item}
            <Sidebar.MenuItem>
              {#if item.isExpandable}
                <Sidebar.MenuButton
                  tooltipContent={item.title}
                  onclick={() => playgroundOpen = !playgroundOpen}
                >
                  {#snippet child({ props })}
                    <button {...props}>
                      <item.icon />
                      <span>{item.title}</span>
                      <ChevronDown class="ml-auto transition-transform duration-200 {playgroundOpen ? 'rotate-180' : ''}" />
                    </button>
                  {/snippet}
                </Sidebar.MenuButton>
                {#if playgroundOpen && item.subitems}
                  <Sidebar.MenuSub>
                    {#each item.subitems as subitem}
                      <Sidebar.MenuSubItem>
                        <Sidebar.MenuSubButton isActive={subitem.url === currentPath}>
                          {#snippet child({ props })}
                            <a href={subitem.url} {...props}>
                              <span>{subitem.title}</span>
                            </a>
                          {/snippet}
                        </Sidebar.MenuSubButton>
                      </Sidebar.MenuSubItem>
                    {/each}
                  </Sidebar.MenuSub>
                {/if}
              {:else}
                <Sidebar.MenuButton 
                  isActive={item.url === currentPath}
                  tooltipContent={item.title}
                >
                  {#snippet child({ props })}
                    <a href={item.url} {...props}>
                      <item.icon />
                      <span>{item.title}</span>
                    </a>
                  {/snippet}
                </Sidebar.MenuButton>
              {/if}
            </Sidebar.MenuItem>
          {/each}
        </Sidebar.Menu>
      </Sidebar.GroupContent>
    </Sidebar.Group>
    
    {#if showAdmin}
      <Sidebar.Group>
        <Sidebar.GroupLabel>Admin</Sidebar.GroupLabel>
        <Sidebar.GroupContent>
          <Sidebar.Menu>
            {#each adminItems as item}
              <Sidebar.MenuItem>
                <Sidebar.MenuButton
                  isActive={item.url === currentPath}
                  tooltipContent={item.title}
                >
                  {#snippet child({ props })}
                    <a href={item.url} {...props}>
                      <item.icon />
                      <span>{item.title}</span>
                    </a>
                  {/snippet}
                </Sidebar.MenuButton>
              </Sidebar.MenuItem>
            {/each}
          </Sidebar.Menu>
        </Sidebar.GroupContent>
      </Sidebar.Group>
    {/if}

    <Sidebar.Group>
      <Sidebar.GroupLabel>Settings</Sidebar.GroupLabel>
      <Sidebar.GroupContent>
        <Sidebar.Menu>
          {#each settingItems as item}
            <Sidebar.MenuItem>
              {#if item.isExpandable}
                <Sidebar.MenuButton
                  tooltipContent={item.title}
                  onclick={() => playgroundOpen = !playgroundOpen}
                >
                  {#snippet child({ props })}
                    <button {...props}>
                      <item.icon />
                      <span>{item.title}</span>
                      <ChevronDown class="ml-auto transition-transform duration-200 {playgroundOpen ? 'rotate-180' : ''}" />
                    </button>
                  {/snippet}
                </Sidebar.MenuButton>
                {#if playgroundOpen && item.subitems}
                  <Sidebar.MenuSub>
                    {#each item.subitems as subitem}
                      <Sidebar.MenuSubItem>
                        <Sidebar.MenuSubButton isActive={subitem.url === currentPath}>
                          {#snippet child({ props })}
                            <a href={subitem.url} {...props}>
                              <span>{subitem.title}</span>
                            </a>
                          {/snippet}
                        </Sidebar.MenuSubButton>
                      </Sidebar.MenuSubItem>
                    {/each}
                  </Sidebar.MenuSub>
                {/if}
              {:else}
                <Sidebar.MenuButton 
                  isActive={item.url === currentPath}
                  tooltipContent={item.title}
                >
                  {#snippet child({ props })}
                    <a href={item.url} {...props}>
                      <item.icon />
                      <span>{item.title}</span>
                    </a>
                  {/snippet}
                </Sidebar.MenuButton>
              {/if}
            </Sidebar.MenuItem>
          {/each}
        </Sidebar.Menu>
      </Sidebar.GroupContent>
    </Sidebar.Group>
  </Sidebar.Content>
  
  <Sidebar.Footer>
    {#if userLoggedIn}
      <Sidebar.Menu>
        <Sidebar.MenuItem>
          <DropdownMenu.Root>
            <DropdownMenu.Trigger>
              {#snippet child({ props })}
                <button 
                  {...props}
                  class="flex w-full items-center gap-2 rounded-md p-2 text-left text-sm outline-none ring-sidebar-ring transition-[width,height,padding] hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 active:bg-sidebar-accent active:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50 group-has-[[data-sidebar=menu-action]]/menu-item:pr-8 aria-disabled:pointer-events-none aria-disabled:opacity-50 data-[active=true]:bg-sidebar-accent data-[active=true]:font-medium data-[active=true]:text-sidebar-accent-foreground data-[state=open]:hover:bg-sidebar-accent data-[state=open]:hover:text-sidebar-accent-foreground group-data-[collapsible=icon]:!size-8 group-data-[collapsible=icon]:!p-2 [&>span:last-child]:truncate [&>svg]:size-4 [&>svg]:shrink-0"
                >
                  <div class="flex aspect-square size-8 items-center justify-center rounded-lg bg-muted">
                    <SquareUser class="size-4" />
                  </div>
                  <div class="grid flex-1 text-left text-sm leading-tight">
                    <span class="truncate font-semibold">{$auth?.name || $auth?.username || 'User'}</span>
                    <span class="truncate text-xs text-muted-foreground">{$auth?.email || ''}</span>
                  </div>
                </button>
              {/snippet}
            </DropdownMenu.Trigger>
            <DropdownMenu.Content class="w-56" side="top" align="end" sideOffset={4}>
              <DropdownMenu.Item 
                onclick={handleSignOut}
                disabled={isSigningOut}
                class="cursor-pointer"
              >
                <LogOut class="mr-2 h-4 w-4" />
                <span>{isSigningOut ? 'Logging out...' : 'Log out'}</span>
              </DropdownMenu.Item>
            </DropdownMenu.Content>
          </DropdownMenu.Root>
        </Sidebar.MenuItem>
      </Sidebar.Menu>
    {/if}
  </Sidebar.Footer>
  
  <Sidebar.Rail />
</Sidebar.Root>
