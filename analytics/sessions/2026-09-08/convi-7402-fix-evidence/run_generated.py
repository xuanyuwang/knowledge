from pathlib import Path
import subprocess
p=Path('/tmp/convi7402-fixed')
ddl="""CREATE TABLE moment_annotation_mv_by_metadata_d (conversation_id String,conversation_start_time DateTime64(6), moment_template_id String,metadata_string_value String,metadata_number_value Float64,metadata_bool_value Bool,update_time DateTime64(6)) ENGINE=Memory;
CREATE TABLE moment_annotation_d (conversation_id String, conversation_start_time DateTime64(6),conversation_end_time DateTime64(6),moment_template_id String,moment_type Int32,metadata_string_value String,metadata_number_value Float64,metadata_bool_value Bool,create_time DateTime64(6)) ENGINE=Memory;
CREATE TABLE base (conversation_id String) ENGINE=Memory;
INSERT INTO base VALUES ('missing'),('selected'),('other'),('five'),('ten'),('six');
INSERT INTO moment_annotation_mv_by_metadata_d VALUES
('selected','2020-12-01','A','X',0,false,'2021-01-02'),
('other','2020-12-01','A','Y',0,false,'2021-01-02'),
('five','2020-12-01','A','',5,false,'2021-01-02'),
('ten','2020-12-01','A','',10,false,'2021-01-02'),
('six','2020-12-01','A','',6,false,'2021-01-02');
INSERT INTO moment_annotation_d SELECT conversation_id,conversation_start_time,conversation_start_time,moment_template_id,19,metadata_string_value,metadata_number_value,metadata_bool_value,update_time FROM moment_annotation_mv_by_metadata_d;
"""
for file in sorted(p.glob('generated-*.sql')):
 full=p/('execute-'+file.name);full.write_text(ddl+file.read_text()+';')
 r=subprocess.run(['clickhouse','local','--multiquery','--queries-file',str(full)],capture_output=True,text=True)
 (p/(file.stem+'.result')).write_text(r.stdout+r.stderr)
 print(file.name,'exit',r.returncode,'rows',r.stdout.strip().splitlines(),'error',r.stderr[:400])
