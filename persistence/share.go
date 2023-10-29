package persistence

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type Share struct {
	PoolID            string
	BlockHeight       uint
	Miner             string
	Worker            string
	UserAgent         string
	Difficulty        float64
	NetworkDifficulty float64
	IpAddress         string
	Created           time.Time
}

type ShareRepository struct {
	*sql.DB
}

func (r *ShareRepository) InsertBatch(shares []Share) error {
	txn, err := r.DB.Begin()
	if err != nil {
		return err
	}

	fields := pq.CopyIn("shares", "poolid", "blockheight", "difficulty", "networkdifficulty",
		"miner", "worker", "useragent", "ipaddress", "source", "created")
	stmt, err := txn.Prepare(fields)
	if err != nil {
		return err
	}

	for _, share := range shares {
		_, err = stmt.Exec(share.PoolID, share.BlockHeight, share.Difficulty,
			share.NetworkDifficulty, share.Miner, share.Worker, share.UserAgent, share.IpAddress,
			"", share.Created)
		if err != nil {
			return err
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	err = stmt.Close()
	if err != nil {
		return err
	}

	return txn.Commit()
}

func (r *ShareRepository) GetSharesBefore(poolID string, before time.Time, inclusive bool, pageSize int) ([]Share, error) {
	query := "SELECT poolid, blockheight, difficulty, networkdifficulty, miner, worker, useragent, ipaddress, created "
	query = query + "FROM shares WHERE poolid = ? AND created %v ? ORDER BY created DESC FETCH NEXT ? ROWS ONLY"
	operator := "<"
	if inclusive {
		operator = "<="
	}
	query = fmt.Sprintf(query, operator)

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return nil, err
	}

	var shares []Share
	rows, err := stmt.Query(poolID, before, pageSize)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var share Share

		err = rows.Scan(&share.PoolID, &share.BlockHeight, &share.Difficulty, &share.NetworkDifficulty,
			&share.Miner, &share.Worker, &share.UserAgent, &share.IpAddress, &share.Created)
		if err != nil {
			return nil, err
		}

		shares = append(shares, share)
	}

	return shares, nil
}

func (r *ShareRepository) CountSharesBefore(poolID string, before time.Time, inclusive bool) (uint, error) {
	query := "SELECT count(*) FROM shares WHERE poolid = ? AND created < ?"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return 0, err
	}

	var count uint
	err = stmt.QueryRow(poolID, before).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *ShareRepository) CountSharesByMiner(poolID, minerAddress string) (uint, error) {
	query := "SELECT count(*) FROM shares WHERE poolid = ? AND miner = ?"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return 0, err
	}

	var count uint
	err = stmt.QueryRow(poolID, minerAddress).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *ShareRepository) GetEffortBetweenCreated(poolID string, shareConst float64, start, end time.Time) (float64, error) {
	query := "SELECT SUM((difficulty * ?) / networkdifficulty) FROM shares WHERE poolid = ? AND created > ? AND created < ?"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return 0, err
	}

	var effort float64
	err = stmt.QueryRow(shareConst, poolID, start, end).Scan(&effort)
	if err != nil {
		return 0, err
	}

	return effort, nil
}

func (r *ShareRepository) DeleteSharesByMiner(poolID, minerAddress string) error {
	query := "DELETE FROM shares WHERE poolid = ? AND miner = ?"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(poolID, minerAddress)
	return err
}

func (r *ShareRepository) DeleteSharesBefore(poolID string, before time.Time) error {
	query := "DELETE FROM shares WHERE poolid = ? AND created < ?"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(poolID, before)
	return err
}

func (r *ShareRepository) GetAccumulatedShareDifficultyBetween(poolID string, start, end time.Time) (float64, error) {
	query := "SELECT SUM(difficulty) FROM shares WHERE poolid = ? AND created > ? AND created < ?"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return 0, err
	}

	var difficulty float64
	err = stmt.QueryRow(poolID, start, end).Scan(&difficulty)
	if err != nil {
		return 0, err
	}

	return difficulty, nil
}

func (r *ShareRepository) GetEffectiveAccumulatedShareDifficultyBetween(poolID string, start, end time.Time) (float64, error) {
	query := "SELECT SUM(difficulty / networkdifficulty) FROM shares WHERE poolid = ? AND created > ? AND created < ?"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return 0, err
	}

	var difficulty float64
	err = stmt.QueryRow(poolID, start, end).Scan(&difficulty)
	if err != nil {
		return 0, err
	}

	return difficulty, nil
}

type MinerWorkerHashSummary struct {
	Miner      string
	Worker     string
	Difficulty string
	ShareCount uint
	FirstShare time.Time
	LastShare  time.Time
}

func (r *ShareRepository) GetHashAccumulationBetween(poolID string, start, end time.Time) (MinerWorkerHashSummary, error) {
	hashSummary := MinerWorkerHashSummary{}

	query := "SELECT SUM(difficulty), COUNT(difficulty), MIN(created) AS firstshare, MAX(created) AS lastshare, miner, worker "
	query = query + "FROM shares WHERE poolid = ? AND created >= ? AND created <= ? GROUP BY miner, worker"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return hashSummary, err
	}

	err = stmt.QueryRow(poolID, start, end).Scan(&hashSummary.Difficulty, &hashSummary.ShareCount, &hashSummary.FirstShare,
		&hashSummary.LastShare, &hashSummary.Miner, &hashSummary.Worker)
	if err != nil {
		return hashSummary, err
	}

	return hashSummary, nil
}

type UserAgentShareDifficultyMap map[string]float64 // UserAgent => Difficulty

func (r *ShareRepository) GetAccumulatedUserAgentShareDifficultyBetween(poolID string, start, end time.Time, byVersion bool) (UserAgentShareDifficultyMap, error) {
	userAgentDiffMap := make(UserAgentShareDifficultyMap)

	userAgentString := "REGEXP_REPLACE(useragent, '/.+', '')"
	if byVersion {
		userAgentString = "useragent"
	}

	query := "SELECT SUM(difficulty) AS value, %v AS key FROM shares "
	query = fmt.Sprint(query, userAgentString)
	query = query + "WHERE poolid = ? AND created > ? AND created < ? GROUP BY key ORDER BY value DESC"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return userAgentDiffMap, err
	}

	rows, err := stmt.Query(poolID, start, end)
	if err != nil {
		return userAgentDiffMap, err
	}

	if err != nil {
		return userAgentDiffMap, err
	}

	for rows.Next() {
		var userAgent string
		var difficulty float64
		err = rows.Scan(&difficulty, &userAgent)
		if err != nil {
			return userAgentDiffMap, err
		}

		userAgentDiffMap[userAgent] = difficulty
	}

	return userAgentDiffMap, nil
}

func (r *ShareRepository) GetRecentyUsedIpAddresses(poolID string) ([]string, error) {
	query := "SELECT DISTINCT s.ipaddress FROM shares WHERE poolid = ? ORDER BY CREATED DESC LIMIT 100"

	stmt, err := r.DB.Prepare(query)
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query(poolID)
	if err != nil {
		return nil, err
	}

	var ips []string
	for rows.Next() {
		var ip string
		err = rows.Scan(&ip)
		if err != nil {
			return ips, err
		}

		ips = append(ips, ip)
	}

	return ips, nil
}
