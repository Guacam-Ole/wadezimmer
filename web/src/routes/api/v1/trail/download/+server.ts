import { handleError } from "$lib/util/api_util";
import { DownloadError, fetchExternalFile } from "$lib/util/secure_fetch_util";
import { json, type RequestEvent } from "@sveltejs/kit";

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

    // Follow redirect for short URLs (bkrtr.de) to get the full bikerouter.de URL with hash
    if (new URL(url).hostname === 'bkrtr.de') {
        const response = await fetch(url, { redirect: 'manual' });
        const location = response.headers.get('Location');
        if (!location) {
            throw new Error('Could not resolve bkrtr.de short URL - no Location header in redirect');
        }
        bikerouterUrl = location;
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

      const apiUrl = `https://brouter.m11n.de/brouter-engine?lonlats=${encodeURIComponent(apiLonlats)}&profile=${encodeURIComponent(profile)}&alternativeidx=0&format=gpx`;
    const gpxResponse = await fetch(apiUrl, { redirect: 'follow' });

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

/**
 * @swagger
 * /api/v1/trail/download:
 *   post:
 *     summary: Download file from URL
 *     description: Downloads a file from a URL and returns it as a blob
 *     tags:
 *       - Trails
 *     requestBody:
 *       required: true
 *       content:
 *         application/json:
 *           schema:
 *             type: object
 *             required:
 *               - url
 *             properties:
 *               url:
 *                 type: string
 *                 format: uri
 *     responses:
 *       200:
 *         description: File blob
 *         content:
 *           application/octet-stream:
 *             schema:
 *               type: string
 *               format: binary
 *       400:
 *         description: Bad Request
 *       401:
 *         description: Unauthorized
 *       500:
 *         description: Internal Server Error
 */
export async function POST(event: RequestEvent) {
    if (!event.locals.user) {
        return json({ message: "Unauthorized" }, { status: 401 });
    }

    try {
        const data = await event.request.json();

        if (isBikerouterUrl(data.url)) {
            return await fetchBikerouterGpx(data.url);
        }

        const { contentType, body } = await fetchExternalFile(data.url);

        return new Response(new Uint8Array(body), {
            headers: {
                "Content-Type": contentType ?? "application/octet-stream",
            },
        });
    } catch (e) {
        if (e instanceof DownloadError) {
            return json({ message: e.message }, { status: e.status });
        }
        console.error('[download]', e);
        return handleError(e);
    }
}