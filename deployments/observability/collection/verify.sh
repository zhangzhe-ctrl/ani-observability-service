#!/usr/bin/env bash
# Read-only acceptance. Run on the approved Fedora execution host.
set -euo pipefail
: "${PROMETHEUS_URL:?set an accessible Prometheus HTTP URL}"
: "${CLUSTER_ID:?set the cluster UUID read from Resource}"
: "${TENANT_ID:?set the persisted Resource tenant UUID}"
: "${INSTANCE_ID:?set the persisted Resource instance UUID}"
: "${TARGET_NAMESPACE:?set the persisted Resource namespace}"
: "${KUBECONFIG:?reference an approved kubeconfig without printing its contents}"
export KUBECTL="${KUBECTL:-kubectl}"
export CONTEXT="${CONTEXT:-kubernetes-admin@ani-lab}"
export COLLECTION_NAMESPACE="${COLLECTION_NAMESPACE:-ani-observability}"
export EVIDENCE_DIR="${EVIDENCE_DIR:-$(dirname "$0")/artifacts/manual-metrics/$(date -u +%Y%m%dT%H%M%SZ)}"
export SNAPSHOTS="${SNAPSHOTS:-3}"
export SAMPLE_INTERVAL_SECONDS="${SAMPLE_INTERVAL_SECONDS:-31}"
mkdir -p "$EVIDENCE_DIR"
python3 - <<'PY'
import collections, datetime, json, os, pathlib, re, subprocess, time, urllib.parse, urllib.request

out = pathlib.Path(os.environ['EVIDENCE_DIR'])
cluster, tenant, instance, ns = [os.environ[k] for k in ['CLUSTER_ID','TENANT_ID','INSTANCE_ID','TARGET_NAMESPACE']]
expected_api='https://172.16.101.10:6443'
kubectl=[os.environ['KUBECTL'],'--kubeconfig',os.environ['KUBECONFIG'],'--context',os.environ['CONTEXT']]
def kube(*args):
    return json.loads(subprocess.check_output(kubectl+list(args),text=True))
def save(name,value):
    (out/name).write_text(json.dumps(value,indent=2)+'\n')
def require(ok,message):
    if not ok: raise RuntimeError(message)
server=subprocess.check_output(kubectl+['config','view','--minify','-o','jsonpath={.clusters[0].cluster.server}'],text=True)
require(server==expected_api,'API server is outside approved cluster 10-12')
nodes=kube('get','nodes','-o','json')
ips={a['address'] for n in nodes['items'] for a in n['status']['addresses'] if a['type']=='InternalIP'}
require(ips=={'172.16.101.10','172.16.101.11','172.16.101.12'},'node identities do not match approved cluster 10-12')
save('cluster-identity.json',{'context':os.environ['CONTEXT'],'server':expected_api,'node_internal_ips':sorted(ips)})
pods=kube('-n',ns,'get','pods','-l','ani.io/instance-id='+instance,'-o','json')
require(len(pods['items'])==1,'live verification needs exactly one normal Resource instance Pod')
pod=pods['items'][0]; labels=pod['metadata'].get('labels',{})
require(labels.get('ani.io/tenant-id')==tenant,'Pod owner tenant differs from persisted scope')
statuses=[c for c in pod['status'].get('containerStatuses',[]) if c['name']=='workspace']
require(len(statuses)==1 and statuses[0].get('state',{}).get('running'),'workspace must actually be running')
uid=pod['metadata']['uid']; rawcid=statuses[0].get('containerID','')
require(re.fullmatch(r'containerd://[0-9a-f]{64}',rawcid),'unrecognized current runtime CID')
cid=rawcid.removeprefix('containerd://')
save('resource-pod-identity.json',{'namespace':ns,'pod':pod['metadata']['name'],'pod_uid':uid,'runtime_container_id':cid,'labels':{k:labels[k] for k in ['ani.io/tenant-id','ani.io/instance-id']}})
base=os.environ['PROMETHEUS_URL'].rstrip('/')
def get(path,params=None):
    url=base+path+('?' + urllib.parse.urlencode(params) if params else '')
    with urllib.request.urlopen(url,timeout=10) as response: data=json.load(response)
    require(data.get('status')=='success','Prometheus API failed: '+path)
    require(not data.get('warnings'),'Prometheus warnings: '+path)
    return data
def query(expr,t): return get('/api/v1/query',{'query':expr,'time':str(t)})
sd=re.compile(r'/kubepods\.slice/(?:kubepods-(?:besteffort|burstable)\.slice/)?kubepods(?:-(?:besteffort|burstable))?-pod([0-9a-f]{8})_([0-9a-f]{4})_([0-9a-f]{4})_([0-9a-f]{4})_([0-9a-f]{12})\.slice/cri-containerd-([0-9a-f]{64})\.scope')
cg=re.compile(r'/kubepods/(?:besteffort/|burstable/)?pod([0-9a-f-]{36})/([0-9a-f]{64})')
def parse_id(value):
    m=sd.fullmatch(value)
    if m: return '-'.join(m.groups()[:5]),m.group(6)
    m=cg.fullmatch(value)
    if m: return m.group(1),m.group(2)
    return None
seven={'__name__','ani_cluster_id','ani_tenant_id','ani_instance_id','namespace','pod_uid','runtime_container_id','container'}
summary=[]
for sample in range(int(os.environ['SNAPSHOTS'])):
    t=int(time.time()); prefix=f'{sample+1:02d}'
    targets=get('/api/v1/targets'); save(prefix+'-targets.json',targets)
    relevant=[x for x in targets['data']['activeTargets'] if x['labels'].get('job')=='kube-state-metrics' or x.get('scrapeUrl','').endswith('/metrics/cadvisor')]
    require(len(relevant)==4,'expected one KSM and three cAdvisor targets')
    require(all(x['health']=='up' and x.get('scrapeInterval')=='15s' for x in relevant),'selected scrape targets must be up with 15s intervals')
    rules=get('/api/v1/rules'); save(prefix+'-rules.json',rules)
    group=[g for g in rules['data']['groups'] if g['name']=='ani-instance-metrics-v1']
    require(len(group)==1 and len(group[0]['rules'])==12,'all twelve ordered recording rules must load')
    require(all(r.get('health')=='ok' and not r.get('lastError') for r in group[0]['rules']),'recording rule evaluation failed')
    network_names=('container_network_receive_bytes_total','container_network_transmit_bytes_total')
    raw_names=['container_cpu_usage_seconds_total','container_memory_working_set_bytes','kube_pod_container_info','kube_pod_container_status_running','kube_pod_labels']
    raw_names.extend(network_names)
    for name in raw_names:
        selector=name+'{namespace='+json.dumps(ns)+',pod='+json.dumps(pod['metadata']['name'])+'}'
        if name in network_names:
            # Pod netns series carry no container label and their id points at the pause sandbox.
            selector=selector[:-1]+',interface="eth0"}'
        elif name=='container_cpu_usage_seconds_total': selector=selector[:-1]+',container="workspace",cpu="total"}'
        elif name!='kube_pod_labels': selector=selector[:-1]+',container="workspace"}'
        data=query(selector,t); ages=query('time()-timestamp('+selector+')',t)
        save(prefix+'-'+name+'.json',data); save(prefix+'-'+name+'-ages.json',ages)
        rows=data['data']['result']; require(len(rows)==1,name+' must have exactly one raw source before transport label projection')
        metric=rows[0]['metric']; require(metric.get('ani_cluster_id')==cluster,name+' has wrong cluster label')
        require(len(ages['data']['result'])==1 and 0<=float(ages['data']['result'][0]['value'][1])<=45,name+' is stale')
        if name in network_names:
            # Pod-level series: compare the pod UID parsed from the cgroup id only; never the sandbox CID.
            # len(rows)==1 above already proves no more than one eth0 series for this 4-key pod identity.
            parsed=parse_id(metric.get('id',''))
            require(parsed is not None and parsed[0]==uid,name+' Pod UID differs from current Resource Pod')
        elif name.startswith('container_'):
            require(parse_id(metric.get('id',''))==(uid,cid),name+' UID/CID differs from current Resource Pod')
        else:
            require(metric.get('uid')==uid,name+' Pod UID differs')
        if name=='kube_pod_container_info': require(metric.get('container_id')==rawcid,'KSM CID differs')
        if name=='kube_pod_container_status_running': require(float(rows[0]['value'][1])==1,'KSM workspace is not running')
        if name=='kube_pod_labels':
            require(metric.get('label_ani_io_tenant_id')==tenant and metric.get('label_ani_io_instance_id')==instance,'KSM owner labels differ from persisted scope')
    values={}
    for name in ['ani_instance_cpu_usage_cores','ani_instance_memory_working_set_bytes','ani_instance_network_receive_bytes_per_second','ani_instance_network_transmit_bytes_per_second']:
        selector=name+'{ani_cluster_id='+json.dumps(cluster)+',ani_tenant_id='+json.dumps(tenant)+',ani_instance_id='+json.dumps(instance)+',namespace='+json.dumps(ns)+',container="workspace"}'
        data=query(selector+' and (time()-timestamp('+selector+')<=45)',t); save(prefix+'-'+name+'.json',data)
        rows=data['data']['result']; require(len(rows)==1,name+' needs exactly one current canonical source')
        metric=rows[0]['metric']; require(set(metric)==seven,name+' has noncanonical or missing labels')
        require(metric['pod_uid']==uid and metric['runtime_container_id']==cid,name+' has wrong runtime identity')
        values[name]=float(rows[0]['value'][1])
    summary.append({'evaluated_at':datetime.datetime.fromtimestamp(t,datetime.timezone.utc).isoformat(),'values':values,'status':'PASS'})
    if sample+1<int(os.environ['SNAPSHOTS']): time.sleep(int(os.environ['SAMPLE_INTERVAL_SECONDS']))
save('verification-summary.json',{'scope':{'cluster_id':cluster,'tenant_id':tenant,'instance_id':instance,'namespace':ns},'snapshots':summary,'status':'PASS','limitations':['No workload lifecycle or load is performed by this read-only script.','Prometheus tenant network isolation is deferred by explicit user scope.']})
print(json.dumps({'status':'PASS','evidence_dir':str(out),'snapshots':len(summary)}))
PY
