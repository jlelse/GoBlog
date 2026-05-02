package main

import (
	"database/sql"
	"strings"
)

type webmentionBlocklistEntry struct {
	Host     string
	Incoming bool
	Outgoing bool
}

func (a *goBlog) getWebmentionBlocklist() ([]*webmentionBlocklistEntry, error) {
	rows, err := a.db.Query("select host, incoming, outgoing from webmention_blocklist order by host")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []*webmentionBlocklistEntry{}
	for rows.Next() {
		e := &webmentionBlocklistEntry{}
		if err = rows.Scan(&e.Host, &e.Incoming, &e.Outgoing); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (a *goBlog) addWebmentionBlocklistEntry(host string, incoming, outgoing bool) error {
	_, err := a.db.Exec(
		"insert into webmention_blocklist (host, incoming, outgoing) values (@host, @incoming, @outgoing) on conflict (host) do update set incoming = @incoming2, outgoing = @outgoing2",
		sql.Named("host", host),
		sql.Named("incoming", incoming),
		sql.Named("outgoing", outgoing),
		sql.Named("incoming2", incoming),
		sql.Named("outgoing2", outgoing),
	)
	return err
}

func (a *goBlog) removeWebmentionBlocklistEntry(host string) error {
	_, err := a.db.Exec("delete from webmention_blocklist where host = @host", sql.Named("host", host))
	return err
}

func (a *goBlog) isWebmentionBlockedIncoming(host string) bool {
	var blocked bool
	row, err := a.db.QueryRow("select 1 from webmention_blocklist where host = @host and incoming = 1", sql.Named("host", strings.ToLower(host)))
	if err != nil {
		return false
	}
	if err = row.Scan(&blocked); err != nil {
		return false
	}
	return blocked
}

func (a *goBlog) isWebmentionBlockedOutgoing(host string) bool {
	var blocked bool
	row, err := a.db.QueryRow("select 1 from webmention_blocklist where host = @host and outgoing = 1", sql.Named("host", strings.ToLower(host)))
	if err != nil {
		return false
	}
	if err = row.Scan(&blocked); err != nil {
		return false
	}
	return blocked
}
