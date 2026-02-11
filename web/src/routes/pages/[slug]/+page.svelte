<script lang="ts">
    import { page } from "$app/state";
    import { marked } from "marked";
    import { locale } from "svelte-i18n";
    import Scene from "$lib/components/scene.svelte";
    import { Canvas } from "@threlte/core";

    let content: string = $state("");
    let title: string = $derived(page.params.slug ?? "");

    $effect(() => {
        const slug = page.params.slug;
        const loc = $locale ?? "en";
        const prefix = loc.substring(0, 2);
        const candidates = prefix !== loc ? [prefix, loc] : [loc];

        (async () => {
            for (const lang of candidates) {
                try {
                    const res = await fetch(`/md/${lang}/${slug}.md`);
                    if (res.ok) {
                        const text = await res.text();
                        content = await marked.parse(text);
                        return;
                    }
                } catch {
                    // try next candidate
                }
            }
            content = "";
        })();
    });
</script>

<svelte:head>
    <title>{title} | {page.data.appTitle}</title>
</svelte:head>

<section
    class="grid grid-cols-1 lg:grid-cols-2 md:px-8 gap-4 md:gap-8"
    style="min-height: calc(100vh - 112px)"
>
    <div
        class="flex flex-col justify-center gap-8 px-8 md:px-24 mt-0 lg:sticky"
        style="max-height: calc(100vh - 112px); top: 112px;"
    >
        <h2 class="text-4xl md:text-5xl font-bold mt-1 mb-8">
            {title}
        </h2>
        <div class="prose dark:prose-invert">
            {@html content}
        </div>
    </div>
</section>
<div
    class="hidden md:block w-full fixed top-[112px] -z-10"
    style="min-height: calc(100vh - 112px)"
>
    <Canvas toneMapping={0}>
        <Scene></Scene>
    </Canvas>
</div>
