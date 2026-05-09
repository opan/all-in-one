<script lang="ts">
  import { Button } from "$lib/components/ui/button/index";
  import { onMount } from "svelte";

  let theme = $state<string | null>(null);
  let mounted = $state(false);

  onMount(() => {
    const savedTheme = localStorage.getItem("theme") || "light";
    theme = savedTheme;
    applyTheme(savedTheme);
    mounted = true;
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

  function toggleTheme() {
    applyTheme(theme === "dark" ? "light" : "dark");
  }
</script>

<Button variant="ghost" size="icon" onclick={toggleTheme}>
  {#if !mounted || theme === "light"}
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
