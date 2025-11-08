// SvelteKit load function to fetch topic details and items from backend API

export const load = async ({ params, fetch }) => {
  const topicId = params.id;
  
  // Fetch topic details
  const topicRes = await fetch(`/api/v1/topics/${topicId}`);
  if (!topicRes.ok) {
    throw new Error('Failed to fetch topic');
  }
  const topicData = await topicRes.json();
  
  // Fetch items for this topic
  const itemsRes = await fetch(`/api/v1/topics/${topicId}/items`);
  if (!itemsRes.ok) {
    throw new Error('Failed to fetch items');
  }
  const itemsData = await itemsRes.json();
  
  return {
    topic: topicData.data || {},
    items: itemsData.data || []
  };
};
