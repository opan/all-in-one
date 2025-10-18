<script lang="ts">
  import * as Sidebar from "$lib/components/ui/sidebar/index";
  import { House, LayoutDashboard, Settings } from "@lucide/svelte/icons";
  import { page } from "$app/stores";

  const menuItems = [
    {
      title: "Listings",
      url: "/listing",
      icon: House,
    },
    {
      title: "Dashboard",
      url: "/dashboard",
      icon: LayoutDashboard,
    },
    {
      title: "Settings",
      url: "/settings",
      icon: Settings,
    },
  ];
</script>

<Sidebar.Root>
  <Sidebar.Header>
    <div class="flex h-14 items-center px-4">
      <h2 class="text-lg font-semibold">SIDEBAR MENU</h2>
    </div>
  </Sidebar.Header>
  
  <Sidebar.Content>
    <Sidebar.Group>
      <Sidebar.GroupContent>
        <Sidebar.Menu>
          {#each menuItems as item}
            <Sidebar.MenuItem>
              <Sidebar.MenuButton isActive={item.url === $page.url.pathname}>
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
  </Sidebar.Content>
  
  <Sidebar.Rail />
</Sidebar.Root>
