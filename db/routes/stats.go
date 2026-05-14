package routes

import (
	"net/http"

	"github.com/pocketbase/pocketbase/core"
)

type statsMonthStat struct {
	Month         string  `db:"month" json:"month"`
	PublicTrails  int64   `db:"public_trails" json:"publicTrails"`
	PrivateTrails int64   `db:"private_trails" json:"privateTrails"`
	NewDistanceM  float64 `db:"new_distance_m" json:"newDistanceM"`
	ActiveUsers   int64   `db:"active_users" json:"activeUsers"`
}

type statsTotals struct {
	TotalTrails     int64   `db:"total_trails" json:"totalTrails"`
	TotalPublic     int64   `db:"total_public" json:"totalPublic"`
	TotalPrivate    int64   `db:"total_private" json:"totalPrivate"`
	TotalDistanceM  float64 `db:"total_distance_m" json:"totalDistanceM"`
	TotalUsers      int64   `db:"total_users" json:"totalUsers"`
	UsersWithTrails int64   `db:"users_with_trails" json:"usersWithTrails"`
}

func Stats(e *core.RequestEvent) error {
	var byMonth []statsMonthStat
	err := e.App.DB().
		NewQuery(`
			SELECT
				strftime('%Y-%m', created) as month,
				SUM(CASE WHEN public = 1 THEN 1 ELSE 0 END) as public_trails,
				SUM(CASE WHEN public = 0 THEN 1 ELSE 0 END) as private_trails,
				COALESCE(SUM(distance), 0) as new_distance_m,
				COUNT(DISTINCT author) as active_users
			FROM trails
			GROUP BY strftime('%Y-%m', created)
			ORDER BY month ASC
		`).
		All(&byMonth)
	if err != nil {
		return err
	}

	var totals []statsTotals
	err = e.App.DB().
		NewQuery(`
			SELECT
				COUNT(*) as total_trails,
				SUM(CASE WHEN public = 1 THEN 1 ELSE 0 END) as total_public,
				SUM(CASE WHEN public = 0 THEN 1 ELSE 0 END) as total_private,
				COALESCE(SUM(distance), 0) as total_distance_m,
				(SELECT COUNT(*) FROM activitypub_actors WHERE isLocal = 1) as total_users,
				COUNT(DISTINCT author) as users_with_trails
			FROM trails
		`).
		All(&totals)
	if err != nil {
		return err
	}

	var tot statsTotals
	if len(totals) > 0 {
		tot = totals[0]
	}

	return e.JSON(http.StatusOK, map[string]any{
		"byMonth": byMonth,
		"totals":  tot,
	})
}
