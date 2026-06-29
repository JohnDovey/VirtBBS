package fido

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	bbsListDB   *sql.DB
	bbsListDBMu sync.Mutex
)

// BBSListNode is one remote BBS node that has exchanged mail with us.
type BBSListNode struct {
	Network        string `json:"network"`
	NodeAddr       string `json:"node_addr"`
	Name           string `json:"name,omitempty"`
	Location       string `json:"location,omitempty"`
	Sysop          string `json:"sysop,omitempty"`
	EchomailCount  int    `json:"echomail_count"`
	NetmailCount   int    `json:"netmail_count"`
	LastSeen       string `json:"last_seen"`
}

// BBSListUser is a user seen from a remote node.
type BBSListUser struct {
	UserName      string `json:"user_name"`
	UserAddr      string `json:"user_addr"`
	EchomailCount int    `json:"echomail_count"`
	NetmailCount  int    `json:"netmail_count"`
	LastSeen      string `json:"last_seen"`
}

// BBSListPage is a paginated list of nodes.
type BBSListPage struct {
	Nodes []BBSListNode `json:"nodes"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
	Pages int           `json:"pages"`
}

// BBSListNetworkGroup groups nodes under one network name.
type BBSListNetworkGroup struct {
	Network string        `json:"network"`
	Nodes   []BBSListNode `json:"nodes"`
	Total   int           `json:"total"`
	Page    int           `json:"page"`
	Pages   int           `json:"pages"`
}

// BBSListCharts holds time-series data for the BBS List graphs.
type BBSListCharts struct {
	Labels    []string `json:"labels"`
	Echomail  []int    `json:"echomail"`
	Netmail   []int    `json:"netmail"`
	HasData   bool     `json:"has_data"`
}

// InitBBSList attaches the shared database and ensures schema exists.
func InitBBSList(db *sql.DB) {
	bbsListDBMu.Lock()
	bbsListDB = db
	bbsListDBMu.Unlock()
	if db == nil {
		return
	}
	_ = migrateBBSList(db)
}

func migrateBBSList(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS fido_bbs_nodes (
			network TEXT NOT NULL, node_addr TEXT NOT NULL,
			zone INTEGER NOT NULL, net INTEGER NOT NULL, node_num INTEGER NOT NULL,
			echomail_count INTEGER NOT NULL DEFAULT 0, netmail_count INTEGER NOT NULL DEFAULT 0,
			last_seen TEXT NOT NULL, PRIMARY KEY (network, node_addr))`,
		`CREATE INDEX IF NOT EXISTS idx_fido_bbs_nodes_echo ON fido_bbs_nodes(network, echomail_count DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_fido_bbs_nodes_netmail ON fido_bbs_nodes(network, netmail_count DESC)`,
		`CREATE TABLE IF NOT EXISTS fido_bbs_users (
			network TEXT NOT NULL, node_addr TEXT NOT NULL,
			user_name TEXT NOT NULL, user_addr TEXT NOT NULL,
			echomail_count INTEGER NOT NULL DEFAULT 0, netmail_count INTEGER NOT NULL DEFAULT 0,
			last_seen TEXT NOT NULL, PRIMARY KEY (network, user_addr))`,
		`CREATE INDEX IF NOT EXISTS idx_fido_bbs_users_node ON fido_bbs_users(network, node_addr)`,
		`CREATE TABLE IF NOT EXISTS fido_bbs_daily (
			day TEXT NOT NULL, network TEXT NOT NULL,
			echomail_count INTEGER NOT NULL DEFAULT 0, netmail_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, network))`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM fido_bbs_nodes`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		var msgTbl int
		if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='messages'`).Scan(&msgTbl); err == nil && msgTbl > 0 {
			return backfillBBSList(db)
		}
	}
	return nil
}

// RecordTossMessage records one inbound tossed message's from/to addresses.
func RecordTossMessage(db *sql.DB, network string, our Addr, pm *Message, isNetmail bool) {
	if pm == nil || network == "" {
		return
	}
	db = bbsListDBOr(db)
	if db == nil {
		return
	}
	echo := !isNetmail
	recordEnd(db, network, our, pm.OrigAddr, pm.FromName, echo, isNetmail)
	recordEnd(db, network, our, pm.DestAddr, pm.ToName, echo, isNetmail)
}

// RecordOutboundMessage records one outbound scanned/sent message.
func RecordOutboundMessage(db *sql.DB, network string, our, remote Addr, userName string, echo, netmail bool) {
	if network == "" {
		return
	}
	db = bbsListDBOr(db)
	if db == nil {
		return
	}
	recordEnd(db, network, our, remote, userName, echo, netmail)
}

func bbsListDBOr(db *sql.DB) *sql.DB {
	if db != nil {
		return db
	}
	bbsListDBMu.Lock()
	defer bbsListDBMu.Unlock()
	return bbsListDB
}

func recordEnd(db *sql.DB, network string, our, remote Addr, userName string, echo, netmail bool) {
	if remote == (Addr{}) {
		return
	}
	if our != (Addr{}) && remote.BossString() == our.BossString() {
		return
	}
	userName = strings.TrimSpace(userName)
	if userName == "" {
		userName = "Unknown"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	day := time.Now().UTC().Format("2006-01-02")
	boss := remote.BossString()
	userAddr := remote.String()

	echoInc, netmailInc := 0, 0
	if echo {
		echoInc = 1
	}
	if netmail {
		netmailInc = 1
	}

	_, _ = db.Exec(`INSERT INTO fido_bbs_nodes
		(network, node_addr, zone, net, node_num, echomail_count, netmail_count, last_seen)
		VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT(network, node_addr) DO UPDATE SET
			echomail_count = echomail_count + excluded.echomail_count,
			netmail_count = netmail_count + excluded.netmail_count,
			last_seen = excluded.last_seen`,
		network, boss, remote.Zone, remote.Net, remote.Node, echoInc, netmailInc, now)

	_, _ = db.Exec(`INSERT INTO fido_bbs_users
		(network, node_addr, user_name, user_addr, echomail_count, netmail_count, last_seen)
		VALUES (?,?,?,?,?,?,?)
		ON CONFLICT(network, user_addr) DO UPDATE SET
			node_addr = excluded.node_addr,
			user_name = excluded.user_name,
			echomail_count = echomail_count + excluded.echomail_count,
			netmail_count = netmail_count + excluded.netmail_count,
			last_seen = excluded.last_seen`,
		network, boss, userName, userAddr, echoInc, netmailInc, now)

	_, _ = db.Exec(`INSERT INTO fido_bbs_daily (day, network, echomail_count, netmail_count)
		VALUES (?,?,?,?)
		ON CONFLICT(day, network) DO UPDATE SET
			echomail_count = echomail_count + excluded.echomail_count,
			netmail_count = netmail_count + excluded.netmail_count`,
		day, network, echoInc, netmailInc)
}

// ListBBSNodesEchomail returns nodes with echomail activity, paginated.
func ListBBSNodesEchomail(db *sql.DB, page, pageSize int, search string) (*BBSListPage, error) {
	return listBBSNodes(db, "n.echomail_count > 0", "n.echomail_count DESC, n.last_seen DESC", "", page, pageSize, search)
}

// ListBBSNodesNetmail returns nodes with netmail activity, paginated.
func ListBBSNodesNetmail(db *sql.DB, page, pageSize int, search string) (*BBSListPage, error) {
	return listBBSNodes(db, "n.netmail_count > 0", "n.netmail_count DESC, n.last_seen DESC", "", page, pageSize, search)
}

// ListBBSNodesByNetwork returns nodes for one network, paginated.
func ListBBSNodesByNetwork(db *sql.DB, network string, page, pageSize int, search string) (*BBSListPage, error) {
	network = strings.TrimSpace(network)
	if network == "" {
		return &BBSListPage{Nodes: []BBSListNode{}}, nil
	}
	return listBBSNodes(db, "1=1", "n.echomail_count + n.netmail_count DESC, n.last_seen DESC", network, page, pageSize, search)
}

// ListBBSNetworkNames returns distinct network names that have BBS list data.
func ListBBSNetworkNames(db *sql.DB) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}
	rows, err := db.Query(`SELECT DISTINCT network FROM fido_bbs_nodes ORDER BY network`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return out, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func listBBSNodes(db *sql.DB, where, order, network string, page, pageSize int, search string) (*BBSListPage, error) {
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 15
	}
	offset := (page - 1) * pageSize

	netCond := ""
	args := []any{}
	if network != "" {
		netCond = " AND n.network=?"
		args = append(args, network)
	}

	searchCond, searchArgs := bbsListSearchSQL(search)
	args = append(args, searchArgs...)

	countSQL := `SELECT COUNT(*) FROM fido_bbs_nodes n WHERE ` + where + netCond + searchCond
	var total int
	if err := db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, err
	}
	pages := 0
	if total > 0 {
		pages = (total + pageSize - 1) / pageSize
	}

	querySQL := `SELECT n.network, n.node_addr, n.echomail_count, n.netmail_count, n.last_seen
		FROM fido_bbs_nodes n WHERE ` + where + netCond + searchCond +
		` ORDER BY ` + order + ` LIMIT ? OFFSET ?`
	queryArgs := append(args, pageSize, offset)
	rows, err := db.Query(querySQL, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []BBSListNode
	for rows.Next() {
		var n BBSListNode
		if err := rows.Scan(&n.Network, &n.NodeAddr, &n.EchomailCount, &n.NetmailCount, &n.LastSeen); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	ndb := OpenNodelistDB(db)
	for i := range nodes {
		enrichBBSNode(ndb, &nodes[i])
	}
	if nodes == nil {
		nodes = []BBSListNode{}
	}
	return &BBSListPage{Nodes: nodes, Total: total, Page: page, Pages: pages}, nil
}

func bbsListSearchSQL(search string) (string, []any) {
	search = strings.TrimSpace(search)
	if search == "" {
		return "", nil
	}
	like := "%" + search + "%"
	return ` AND (
		n.node_addr LIKE ? OR
		EXISTS (
			SELECT 1 FROM fido_nodes fn
			WHERE fn.zone = n.zone AND fn.net = n.net AND fn.node_num = n.node_num AND fn.point = 0
				AND (fn.network = n.network OR fn.network = '')
				AND (fn.name LIKE ? OR fn.location LIKE ? OR fn.sysop LIKE ?)
		) OR
		EXISTS (
			SELECT 1 FROM fido_bbs_users u
			WHERE u.network = n.network AND u.node_addr = n.node_addr
				AND (u.user_name LIKE ? OR u.user_addr LIKE ?)
		)
	)`, []any{like, like, like, like, like, like}
}

func enrichBBSNode(ndb *NodelistDB, n *BBSListNode) {
	if ndb == nil || n == nil {
		return
	}
	a, err := ParseAddr(n.NodeAddr)
	if err != nil {
		return
	}
	entry, err := ndb.LookupAddr(n.Network, a)
	if err != nil || entry == nil {
		entry, _ = ndb.LookupAddr("", a)
	}
	if entry != nil {
		n.Name = entry.Name
		n.Location = entry.Location
		n.Sysop = entry.Sysop
	}
}

// ListBBSUsersForNode returns users seen from a given node.
func ListBBSUsersForNode(db *sql.DB, network, nodeAddr string) ([]BBSListUser, error) {
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}
	network = strings.TrimSpace(network)
	nodeAddr = strings.TrimSpace(nodeAddr)
	if network == "" || nodeAddr == "" {
		return []BBSListUser{}, nil
	}
	rows, err := db.Query(`SELECT user_name, user_addr, echomail_count, netmail_count, last_seen
		FROM fido_bbs_users WHERE network=? AND node_addr=?
		ORDER BY echomail_count + netmail_count DESC, user_name`,
		network, nodeAddr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BBSListUser
	for rows.Next() {
		var u BBSListUser
		if err := rows.Scan(&u.UserName, &u.UserAddr, &u.EchomailCount, &u.NetmailCount, &u.LastSeen); err != nil {
			return out, err
		}
		out = append(out, u)
	}
	if out == nil {
		out = []BBSListUser{}
	}
	return out, rows.Err()
}

// QueryBBSListCharts returns aggregated daily echomail/netmail counts for graphs.
func QueryBBSListCharts(db *sql.DB, days int) (*BBSListCharts, error) {
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}
	if days < 1 {
		days = 30
	}
	now := time.Now().UTC()
	labels := make([]string, days)
	keys := make([]string, days)
	for i := days - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		labels[days-1-i] = d.Format("Jan 2")
		keys[days-1-i] = d.Format("2006-01-02")
	}
	echo := make([]int, days)
	netmail := make([]int, days)
	keyIndex := make(map[string]int, days)
	for i, k := range keys {
		keyIndex[k] = i
	}

	since := keys[0]
	rows, err := db.Query(`SELECT day, SUM(echomail_count), SUM(netmail_count)
		FROM fido_bbs_daily WHERE day >= ? GROUP BY day ORDER BY day`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hasData := false
	for rows.Next() {
		var day string
		var e, n int
		if err := rows.Scan(&day, &e, &n); err != nil {
			return nil, err
		}
		if idx, ok := keyIndex[day]; ok {
			echo[idx] = e
			netmail[idx] = n
			if e > 0 || n > 0 {
				hasData = true
			}
		}
	}
	return &BBSListCharts{Labels: labels, Echomail: echo, Netmail: netmail, HasData: hasData}, rows.Err()
}

func backfillBBSList(db *sql.DB) error {
	type msgRow struct {
		network, origin, fromName string
		echo, confID              int
	}
	rows, err := db.Query(`SELECT COALESCE(NULLIF(TRIM(fido_network),''), 'FidoNet'),
		fido_origin, from_name, echo, conference_id
		FROM messages WHERE fido_origin IS NOT NULL AND TRIM(fido_origin) != ''`)
	if err != nil {
		return err
	}
	var msgRows []msgRow
	for rows.Next() {
		var r msgRow
		if err := rows.Scan(&r.network, &r.origin, &r.fromName, &r.echo, &r.confID); err != nil {
			rows.Close()
			return err
		}
		msgRows = append(msgRows, r)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, r := range msgRows {
		addr, err := ParseAddr(r.origin)
		if err != nil {
			continue
		}
		isNetmail := r.confID == 0 && r.echo == 0
		recordEnd(db, r.network, Addr{}, addr, r.fromName, r.echo != 0, isNetmail)
	}

	var netmailTbl int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='fido_netmail'`).Scan(&netmailTbl); err != nil || netmailTbl == 0 {
		return nil
	}

	type netmailRow struct {
		network, toAddr, toName string
	}
	nmRows, err := db.Query(`SELECT COALESCE(NULLIF(TRIM(network),''), 'FidoNet'),
		to_addr, to_name FROM fido_netmail WHERE sent_at IS NOT NULL AND TRIM(sent_at) != ''`)
	if err != nil {
		return err
	}
	var netmailRows []netmailRow
	for nmRows.Next() {
		var r netmailRow
		if err := nmRows.Scan(&r.network, &r.toAddr, &r.toName); err != nil {
			nmRows.Close()
			return err
		}
		netmailRows = append(netmailRows, r)
	}
	if err := nmRows.Err(); err != nil {
		nmRows.Close()
		return err
	}
	nmRows.Close()

	for _, r := range netmailRows {
		addr, err := ParseAddr(r.toAddr)
		if err != nil {
			continue
		}
		recordEnd(db, r.network, Addr{}, addr, r.toName, false, true)
	}
	return nil
}
