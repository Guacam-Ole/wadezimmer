import { handleError } from "$lib/util/api_util";
import { type RequestEvent } from "@sveltejs/kit";

function isBikerouterUrl(url: string): boolean {
    try {
        const parsed = new URL(url);
        return parsed.hostname === 'bikerouter.de' ||
            parsed.hostname === 'www.bikerouter.de' ||
            parsed.hostname === 'bkrtr.de';
    } catch {
        return false;
    }
}

async function fetchBikerouterGpx(url: string): Promise<Response> {
    let bikerouterUrl = url;

    // Follow redirect for short URLs (bkrtr.de) to get the full bikerouter.de URL
    if (new URL(url).hostname === 'bkrtr.de') {
        const response = await fetch(url, { redirect: 'follow' });
        bikerouterUrl = response.url;
    }

    // Parse hash fragment for lonlats and profile
    const hashIndex = bikerouterUrl.indexOf('#');
    if (hashIndex === -1) {
        throw new Error('No route data found in bikerouter URL');
    }

    const hash = bikerouterUrl.substring(hashIndex + 1);
    const params = new URLSearchParams(hash);

    const lonlats = params.get('lonlats');
    const profile = params.get('profile') || 'fastbike';

    if (!lonlats) {
        throw new Error('No waypoints (lonlats) found in bikerouter URL');
    }

    // BRouter API uses pipe separator instead of semicolon
    const apiLonlats = lonlats.split(';').join('|');

    const apiUrl = `https://bikerouter.de/brouter?lonlats=${encodeURIComponent(apiLonlats)}&profile=${encodeURIComponent(profile)}&alternativeidx=0&format=gpx`;
    const gpxResponse = await fetch(apiUrl);

    if (!gpxResponse.ok) {
        throw new Error(`BRouter API error: ${gpxResponse.statusText}`);
    }

    const gpxBlob = await gpxResponse.blob();

    return new Response(gpxBlob, {
        headers: {
            'Content-Type': 'application/gpx+xml',
        },
    });
}

export async function POST(event: RequestEvent) {
    const data = await event.request.json();

    try {
        if (isBikerouterUrl(data.url)) {
            return await fetchBikerouterGpx(data.url);
        }

        const response = await event.fetch(data.url);
        const blob = await response.blob();

        const contentType = response.headers.get('Content-Type') || 'application/octet-stream';

        return new Response(blob, {
            headers: {
                'Content-Type': contentType,
            },
        });
    } catch (e) {
        return handleError(e)
    }
}