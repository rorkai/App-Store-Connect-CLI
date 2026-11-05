import type { APIRoute, GetStaticPaths } from 'astro';
import { getCollection, type CollectionEntry } from 'astro:content';

export const getStaticPaths = (async () => {
	const docs = await getCollection('docs');
	return docs.map((entry) => ({ params: { slug: entry.id }, props: { entry } }));
}) satisfies GetStaticPaths;

export const GET: APIRoute<{ entry: CollectionEntry<'docs'> }> = ({ props }) => {
	const { title, description } = props.entry.data;
	const body = (props.entry.body ?? '').replace(/^import .+\n+/gm, '').trim();
	const header = description ? `# ${title}\n\n> ${description}` : `# ${title}`;
	return new Response(`${header}\n\n${body}\n`, {
		headers: { 'Content-Type': 'text/markdown; charset=utf-8' },
	});
};
