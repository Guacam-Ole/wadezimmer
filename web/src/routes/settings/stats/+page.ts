import { type Load } from '@sveltejs/kit';

export const load: Load = async ({ fetch }) => {
    const r = await fetch('/api/v1/stats');
    if (!r.ok) {
        return { stats: { byMonth: [], totals: {} } };
    }
    return { stats: await r.json() };
};
