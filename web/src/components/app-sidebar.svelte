<script lang="ts">
  import type { Component } from 'svelte';
  import * as Sidebar from "$lib/components/ui/sidebar/index";
  import { 
    SquareTerminal, 
    BookOpen, 
    Settings, 
    ChevronDown,
    History,
    Star,
    Bot,
    SquareUser,
    List,
    Table,
  } from "@lucide/svelte/icons";
  import type { IconProps } from '@lucide/svelte';
	import TableBody from '$lib/components/ui/table/table-body.svelte';

  interface Props {
    currentPath?: string;
  }

  type NavItem = {
    title: string;
    url?: string;
    icon: Component<IconProps, {}, "">;
    isExpandable: boolean;
    subitems?: Array<{
      title: string;
      url: string;
      icon: Component<IconProps, {}, "">;
    }>;
  };

  let { currentPath = "/" }: Props = $props();
  let playgroundOpen = $state(true);

  const platformItems: NavItem[] = [
    {
      title: "Listing",
      icon: Table,
      isExpandable: true,
      subitems: [
        { title: "Category", url: "/listing", icon: List },
      ]
    },
    {
      title: "Settings",
      url: "#",
      icon: Settings,
      isExpandable: false,
    },
  ];
</script>

<Sidebar.Root collapsible="icon">
  <Sidebar.Header>
    <Sidebar.Menu>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton size="lg" tooltipContent="Acme Inc">
          {#snippet child({ props })}
            <a href="/" {...props}>
              <div class="flex aspect-square size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
                <SquareTerminal class="size-4" />
              </div>
              <div class="grid flex-1 text-left text-sm leading-tight">
                <span class="truncate font-semibold">Acme Inc</span>
                <span class="truncate text-xs text-muted-foreground">Enterprise</span>
              </div>
            </a>
          {/snippet}
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
    </Sidebar.Menu>
  </Sidebar.Header>
  
  <Sidebar.Content>
    <Sidebar.Group>
      <Sidebar.GroupLabel>Platform</Sidebar.GroupLabel>
      <Sidebar.GroupContent>
        <Sidebar.Menu>
          {#each platformItems as item}
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
    <Sidebar.Menu>
      <Sidebar.MenuItem>
        <Sidebar.MenuButton size="lg" tooltipContent="shadcn (m@example.com)">
          {#snippet child({ props })}
            <button {...props}>
              <div class="flex aspect-square size-8 items-center justify-center rounded-lg bg-muted">
                <SquareUser class="size-4" />
              </div>
              <div class="grid flex-1 text-left text-sm leading-tight">
                <span class="truncate font-semibold">shadcn</span>
                <span class="truncate text-xs text-muted-foreground">m@example.com</span>
              </div>
            </button>
          {/snippet}
        </Sidebar.MenuButton>
      </Sidebar.MenuItem>
    </Sidebar.Menu>
  </Sidebar.Footer>
  
  <Sidebar.Rail />
</Sidebar.Root>
