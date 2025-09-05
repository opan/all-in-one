<script lang="ts">
  export let data;
  const listings = data.listings;
</script>

<h1>Listing Items</h1>
{#if listings.length > 0}
  <table>
    <thead>
      <tr>
        {#each Object.keys(listings[0]) as key}
          <th>{key}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each listings as item}
        <tr>
          {#each Object.values(item) as value}
            <td>{value}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
{:else}
  <p>No listings found.</p>
{/if}
