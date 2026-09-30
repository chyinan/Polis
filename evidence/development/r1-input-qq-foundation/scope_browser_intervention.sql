UPDATE human_interventions
SET mission_id=NULL,affected_scope='company'
WHERE company_id='r1-input-browser' AND id='browser-intervention-seed';
