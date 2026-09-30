// pattern: Imperative Shell
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	dsn := flag.String("dsn", "", "dedicated restored database DSN")
	company := flag.String("company", "", "authoritative company id")
	output := flag.String("output", "", "state export output")
	flag.Parse()
	if *dsn == "" || *company == "" || *output == "" {
		fail("dsn, company and output are required")
	}
	cfg, err := pgxpool.ParseConfig(*dsn)
	if err != nil {
		fail(err.Error())
	}
	if !strings.HasPrefix(cfg.ConnConfig.Database, "polis_r0_") {
		fail("dedicated polis_r0 database required")
	}
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		fail(err.Error())
	}
	defer pool.Close()
	tx, err := pool.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		fail(err.Error())
	}
	defer tx.Rollback(context.Background())
	const query = `SELECT json_build_object(
 'runtime_control',(SELECT json_build_object('incarnation',incarnation) FROM runtime_control WHERE singleton),
 'company',(SELECT json_build_object('id',id,'company_seq',company_seq) FROM companies WHERE id=$1),
 'employees',COALESCE((SELECT json_agg(json_build_object('id',id,'epoch',epoch) ORDER BY id) FROM employees WHERE company_id=$1),'[]'::json),
 'missions',COALESCE((SELECT json_agg(json_build_object('id',id,'state',state,'activation_id',activation_id,'contract',contract) ORDER BY id) FROM missions WHERE company_id=$1),'[]'::json),
 'tasks',COALESCE((SELECT json_agg(json_build_object('id',id,'mission_id',mission_id,'owner',owner,'kind',kind,'state',state,'generation',generation) ORDER BY id) FROM tasks WHERE company_id=$1),'[]'::json),
 'contract_revisions',COALESCE((SELECT json_agg(json_build_object('id',id,'mission_id',mission_id,'revision',revision,'base_revision',base_revision,'endpoint',endpoint,'schema',schema,'digest',digest,'state',state,'proposer',proposer,'accepter',accepter) ORDER BY revision) FROM contract_revisions WHERE company_id=$1),'[]'::json),
 'messages',COALESCE((SELECT json_agg(json_build_object('id',id,'mission_id',mission_id,'task_id',task_id,'sender',sender,'recipient',recipient,'kind',kind,'delivery_state',delivery_state,'contract_revision_id',contract_revision_id,'task_revision',task_revision) ORDER BY id) FROM messages WHERE company_id=$1),'[]'::json),
 'obligations',COALESCE((SELECT json_agg(json_build_object('id',id,'task_id',task_id,'owner',owner,'state',state,'evidence_ref',evidence_ref) ORDER BY id) FROM obligations WHERE company_id=$1),'[]'::json),
 'peer_work_signals',COALESCE((SELECT json_agg(json_build_object('id',id,'message_id',message_id,'obligation_id',obligation_id,'recipient',recipient,'state',state) ORDER BY id) FROM peer_work_signals WHERE company_id=$1),'[]'::json),
 'worker_sessions',COALESCE((SELECT json_agg(json_build_object('id',id,'employee_id',employee_id,'task_id',task_id,'generation',generation,'epoch',epoch,'incarnation',incarnation,'profile',profile,'state',state,'tool_call_limit',tool_call_limit,'tool_calls_used',tool_calls_used,'stop_receipt',stop_receipt) ORDER BY id) FROM worker_sessions WHERE company_id=$1),'[]'::json),
 'worker_workspaces',COALESCE((SELECT json_agg(json_build_object('task_id',task_id,'digest',digest,'revision',revision) ORDER BY task_id) FROM worker_workspaces WHERE company_id=$1),'[]'::json),
 'worker_checks',COALESCE((SELECT json_agg(json_build_object('id',id,'session_id',session_id,'digest',digest,'phase',phase,'passed',passed,'report',report) ORDER BY id) FROM worker_checks WHERE company_id=$1),'[]'::json),
 'worker_checkpoints',COALESCE((SELECT json_agg(json_build_object('id',id,'session_id',session_id,'digest',digest,'data',data) ORDER BY id) FROM worker_checkpoints WHERE company_id=$1),'[]'::json),
 'task_revisions',COALESCE((SELECT json_agg(json_build_object('id',id,'task_id',task_id,'revision',revision,'digest',digest,'contract_revision_id',contract_revision_id,'state',state) ORDER BY id) FROM task_revisions WHERE company_id=$1),'[]'::json),
 'artifacts',COALESCE((SELECT json_agg(json_build_object('id',id,'task_id',task_id,'author',author,'digest',digest,'bytes',bytes,'state',state,'verdict',verdict,'verifier',verifier,'contract',contract) ORDER BY id) FROM artifacts WHERE company_id=$1),'[]'::json),
 'artifact_staging',COALESCE((SELECT json_agg(json_build_object('id',id,'task_id',task_id,'digest',digest) ORDER BY id) FROM artifact_staging WHERE company_id=$1),'[]'::json),
 'artifact_qualifications',COALESCE((SELECT json_agg(json_build_object('artifact_id',artifact_id,'checkpoint_id',checkpoint_id,'workspace_revision',workspace_revision,'workspace_digest',workspace_digest,'contract_revision_id',contract_revision_id,'acceptance_checker_revision',acceptance_checker_revision,'checkpoint_policy_revision',checkpoint_policy_revision,'artifact_eligibility_policy_revision',artifact_eligibility_policy_revision,'contract_supersession_policy_revision',contract_supersession_policy_revision) ORDER BY artifact_id) FROM artifact_qualifications WHERE company_id=$1),'[]'::json),
 'receipts',COALESCE((SELECT json_agg(json_build_object('actor',actor,'key',key,'fingerprint',fingerprint,'result',result) ORDER BY actor,key) FROM receipts WHERE company_id=$1),'[]'::json),
 'events',COALESCE((SELECT json_agg(json_build_object('company_seq',company_seq,'kind',kind,'payload',payload,'observed',observed) ORDER BY company_seq) FROM events WHERE company_id=$1),'[]'::json))`
	var raw []byte
	if err := tx.QueryRow(context.Background(), query, *company).Scan(&raw); err != nil {
		fail(err.Error())
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		fail(err.Error())
	}
	out, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fail(err.Error())
	}
	if err := os.WriteFile(*output, append(out, '\n'), 0o600); err != nil {
		fail(err.Error())
	}
	if err := tx.Commit(context.Background()); err != nil {
		fail(err.Error())
	}
}

func fail(message string) { fmt.Fprintln(os.Stderr, message); os.Exit(1) }
