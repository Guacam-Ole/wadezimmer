<script lang="ts">
    import { page } from "$app/state";
    import Bar from "$lib/vendor/svelte-chartjs/bar.svelte";
    import Line from "$lib/vendor/svelte-chartjs/line.svelte";
    import { _ } from "svelte-i18n";
    import {
        BarElement,
        CategoryScale,
        Chart as ChartJS,
        Legend,
        LinearScale,
        LineElement,
        PointElement,
        Filler,
        Title,
        Tooltip,
    } from "chart.js";
    import { untrack } from "svelte";

    let { data } = $props();

    ChartJS.register(
        Title,
        Tooltip,
        Legend,
        CategoryScale,
        LinearScale,
        BarElement,
        LineElement,
        PointElement,
        Filler,
    );

    type MonthStat = {
        month: string;
        publicTrails: number;
        privateTrails: number;
        newDistanceM: number;
        activeUsers: number;
    };

    type Totals = {
        totalTrails: number;
        totalPublic: number;
        totalPrivate: number;
        totalDistanceM: number;
        totalUsers: number;
        usersWithTrails: number;
    };

    const byMonth: MonthStat[] = untrack(() => data.stats?.byMonth ?? []);
    const totals: Totals = untrack(
        () =>
            data.stats?.totals ?? {
                totalTrails: 0,
                totalPublic: 0,
                totalPrivate: 0,
                totalDistanceM: 0,
                totalUsers: 0,
                usersWithTrails: 0,
            },
    );

    const labels = byMonth.map((s) => s.month);
    const totalDistanceKm = Math.round((totals.totalDistanceM ?? 0) / 1000);

    // Cumulative distance in km
    const cumDistances: number[] = [];
    let cum = 0;
    for (const s of byMonth) {
        cum += (s.newDistanceM ?? 0) / 1000;
        cumDistances.push(Math.round(cum));
    }

    // Fun distance comparisons
    const EARTH_CIRCUMFERENCE_KM = 40_075;
    const MOON_DISTANCE_KM = 384_400;
    const SUN_DISTANCE_KM = 149_600_000;

    const timesAroundEarth = totalDistanceKm / EARTH_CIRCUMFERENCE_KM;
    const timesToMoon = totalDistanceKm / MOON_DISTANCE_KM;
    const timesToSun = totalDistanceKm / SUN_DISTANCE_KM;

    function formatTimes(n: number): string {
        if (n >= 100) return Math.round(n).toLocaleString();
        if (n >= 10) return n.toFixed(1);
        return n.toFixed(2);
    }

    const trailsChartData = {
        labels,
        datasets: [
            {
                label: $_("public"),
                data: byMonth.map((s) => s.publicTrails),
                backgroundColor: "#3388ff",
                borderRadius: 6,
                stack: "trails",
            },
            {
                label: $_("private"),
                data: byMonth.map((s) => s.privateTrails),
                backgroundColor: "#fb8500",
                borderRadius: 6,
                stack: "trails",
            },
        ],
    };

    const distanceChartData = {
        labels,
        datasets: [
            {
                label: $_("total-distance") + " (km)",
                data: cumDistances,
                borderColor: "#06d6a0",
                backgroundColor: "#06d6a020",
                fill: true,
                tension: 0.3,
                pointRadius: 3,
            },
        ],
    };

    const usersChartData = {
        labels,
        datasets: [
            {
                label: $_("active-users"),
                data: byMonth.map((s) => s.activeUsers),
                borderColor: "#8ecae6",
                backgroundColor: "#8ecae630",
                fill: true,
                tension: 0.3,
                pointRadius: 3,
            },
        ],
    };

    const lineOptions = {
        responsive: true,
        plugins: { legend: { display: false } },
        scales: { y: { beginAtZero: true } },
    };
</script>

<svelte:head>
    <title>{$_("statistics")} | {page.data.appTitle}</title>
</svelte:head>

<div class="max-w-6xl mx-auto px-4 py-8 space-y-6">
    <h2 class="text-2xl font-bold">
        <i class="fa fa-chart-line mr-3"></i>{$_("instance-stats")}
    </h2>

    <div class="grid grid-cols-2 lg:grid-cols-3 gap-4">
        <div
            class="flex flex-col items-center gap-2 border border-input-border rounded-xl p-6"
        >
            <span class="text-gray-500 text-sm font-semibold self-start"
                ><i class="fa fa-users mr-2"></i>{$_("total-users")}</span
            >
            <p class="text-3xl font-bold">{totals.totalUsers}</p>
        </div>
        <div
            class="flex flex-col items-center gap-2 border border-input-border rounded-xl p-6"
        >
            <span class="text-gray-500 text-sm font-semibold self-start"
                ><i class="fa fa-route mr-2"></i>{$_("trail", {
                    values: { n: 2 },
                })}</span
            >
            <p class="text-3xl font-bold">{totals.totalTrails}</p>
            <p class="text-xs text-gray-400">
                <span class="text-blue-400">{totals.totalPublic} {$_("public")}</span>
                ·
                <span class="text-orange-400">{totals.totalPrivate} {$_("private")}</span>
            </p>
        </div>
        <div
            class="flex flex-col items-center gap-2 border border-input-border rounded-xl p-6 col-span-2 lg:col-span-1"
        >
            <span class="text-gray-500 text-sm font-semibold self-start"
                ><i class="fa fa-left-right mr-2"></i>{$_("total-distance")}</span
            >
            <p class="text-3xl font-bold">{totalDistanceKm.toLocaleString()} km</p>
        </div>
    </div>

    {#if timesAroundEarth >= 1 || timesToMoon >= 1 || timesToSun >= 1}
        <div class="border border-input-border rounded-xl p-6 space-y-3">
            <span class="text-gray-500 font-semibold text-lg"
                ><i class="fa fa-earth-europe mr-3"></i>{$_("distance-fun-facts")}</span
            >
            <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 pt-1">
                {#if timesAroundEarth >= 1}
                    <div class="flex items-center gap-4">
                        <span class="text-4xl">🌍</span>
                        <div>
                            <p class="text-2xl font-bold">{formatTimes(timesAroundEarth)}×</p>
                            <p class="text-sm text-gray-500">{$_("times-around-earth")}</p>
                        </div>
                    </div>
                {/if}
                {#if timesToMoon >= 1}
                    <div class="flex items-center gap-4">
                        <span class="text-4xl">🌙</span>
                        <div>
                            <p class="text-2xl font-bold">{formatTimes(timesToMoon)}×</p>
                            <p class="text-sm text-gray-500">{$_("times-to-moon")}</p>
                        </div>
                    </div>
                {/if}
                {#if timesToSun >= 1}
                    <div class="flex items-center gap-4">
                        <span class="text-4xl">☀️</span>
                        <div>
                            <p class="text-2xl font-bold">{formatTimes(timesToSun)}×</p>
                            <p class="text-sm text-gray-500">{$_("times-to-sun")}</p>
                        </div>
                    </div>
                {/if}
            </div>
        </div>
    {/if}

    {#if byMonth.length > 0}
        <div class="border border-input-border rounded-xl p-6 space-y-3">
            <span class="text-gray-500 font-semibold text-lg"
                ><i class="fa fa-route mr-3"></i>{$_("new-trails-per-month")}</span
            >
            <Bar
                data={trailsChartData}
                options={{
                    responsive: true,
                    plugins: { legend: { position: "bottom" } },
                    scales: { x: { stacked: true }, y: { beginAtZero: true, stacked: true } },
                }}
            />
        </div>

        <div class="border border-input-border rounded-xl p-6 space-y-3">
            <span class="text-gray-500 font-semibold text-lg"
                ><i class="fa fa-left-right mr-3"></i>{$_("total-distance")} (km)</span
            >
            <Line data={distanceChartData} options={lineOptions} />
        </div>

        <div class="border border-input-border rounded-xl p-6 space-y-3">
            <span class="text-gray-500 font-semibold text-lg"
                ><i class="fa fa-users mr-3"></i>{$_("active-users-per-month")}</span
            >
            <Line data={usersChartData} options={lineOptions} />
        </div>
    {:else}
        <p class="text-gray-500 text-center py-12">{$_("no-data")}</p>
    {/if}
</div>
