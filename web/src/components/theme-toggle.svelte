<script lang="ts">
  import { Button } from "$lib/components/ui/button/index";
  import * as DropdownMenu from "$lib/components/ui/dropdown-menu/index";
  import { onMount } from "svelte";

  let theme = $state<string>("light");

  onMount(() => {
    const savedTheme = localStorage.getItem("theme") || "light";
    theme = savedTheme;
    applyTheme(savedTheme);
  });

  function applyTheme(newTheme: string) {
    const root = document.documentElement;
    if (newTheme === "dark") {
      root.classList.add("dark");
    } else {
      root.classList.remove("dark");
    }
    localStorage.setItem("theme", newTheme);
    theme = newTheme;
  }

  function setTheme(newTheme: string) {
    applyTheme(newTheme);
  }
</script>

<DropdownMenu.Root>
  <DropdownMenu.Trigger>
    <Button variant="ghost" size="icon">
      {#if theme === "light"}
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          class="h-5 w-5"
        >
          <circle cx="12" cy="12" r="4"></circle>
          <path d="M12 2v2"></path>
          <path d="M12 20v2"></path>
          <path d="m4.93 4.93 1.41 1.41"></path>
          <path d="m17.66 17.66 1.41 1.41"></path>
          <path d="M2 12h2"></path>
          <path d="M20 12h2"></path>
          <path d="m6.34 17.66-1.41 1.41"></path>
          <path d="m19.07 4.93-1.41 1.41"></path>
        </svg>
      {:else}
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          class="h-5 w-5"
        >
          <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z"></path>
        </svg>
      {/if}
      <span class="sr-only">Toggle theme</span>
    </Button>
  </DropdownMenu.Trigger>
  <DropdownMenu.Content align="end">
    <DropdownMenu.Item onclick={() => setTheme("light")}>Light</DropdownMenu.Item>
    <DropdownMenu.Item onclick={() => setTheme("dark")}>Dark</DropdownMenu.Item>
  </DropdownMenu.Content>
</DropdownMenu.Root>
