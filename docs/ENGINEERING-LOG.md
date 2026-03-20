# Engineering Log — RKE2 auf Swiss OTC

> Bekannte Probleme, Einschränkungen und Workarounds aus dem Betrieb.

---

## 🔴 Kritische Probleme (gelöst)

### ELB Subnet-Timeout beim Destroy
**Symptom:** `error deleting OpenTelekomCloud Subnet: timeout while waiting for state to become 'DELETED'`
**Ursache:** CCM-managed ELB hatte noch Members beim Terraform Destroy → Subnet konnte nicht gelöscht werden
**Fix:** Pre-Destroy Step löscht alle `k8s-kubernetes-*` ELBs direkt via OTC ELBv3 API (nicht via CCM)
**Commit:** `80e4fd4`, `af546c7`

---

### cloud-init bricht ab wegen geesefs Download
**Symptom:** `Failed to run module scripts-user` in cloud-init, RKE2 startet nicht
**Ursache:** `set -euo pipefail` + curl-Fehler beim geesefs Download (OBS kein direktes Internet von Nodes) → Exit Code ≠ 0 → cloud-init Abort
**Fix:** geesefs aus cloud-init entfernt. Pipeline installiert geesefs via SCP über Bastion auf alle Nodes
**Commit:** `542470b`

---

### Worker joinen nicht (cloud-init Syntax Error)
**Symptom:** `syntax error near unexpected token '('` in `/var/lib/cloud/instance/scripts/part-001`
**Ursache 1:** `$$(seq 1 30)` in `worker-init.sh.tpl` — Terraform `templatefile()` expandiert `$$` nur in `${...}` Kontext, nicht in `$(...)` → bleibt als `$$` → Bash Syntax Error
**Ursache 2:** `%{http_code}` — Terraform Template Directive Syntax → muss `%%{http_code}` escaped werden
**Fix:** `$$(seq 1 30)` → `$(seq 1 30)`, `%{http_code}` → `%%{http_code}`
**Commits:** `2e2bb74`, `ca1ba79`

---

### EIP 409 beim Apply-Retry
**Symptom:** `HTTP 409 - EIP already in use` beim zweiten Apply-Run kurz nach Destroy
**Ursache:** OTC EIP Release dauert 2-3 Minuten nach Destroy — beim sofortigen Retry noch belegt
**Workaround:** 3-5 Minuten zwischen Destroy und Apply warten

---

## 🟡 Einschränkungen

### geesefs SCP-Install via Pipeline
**Problem:** SSH-Key via `ProxyCommand` + SCP funktioniert nicht zuverlässig in der Pipeline (Key-Format Issue)
**Auswirkung:** geesefs nach neuem Apply nicht auf Nodes → CSI-S3 OBS Mounts schlagen fehl
**Workaround:** Manuell via Bastion installieren:
```bash
scp -o ProxyJump=ubuntu@BASTION_IP -i key geesefs ubuntu@NODE_IP:/tmp/
ssh ubuntu@BASTION_IP "ssh ubuntu@NODE_IP 'sudo install -m755 /tmp/geesefs /usr/local/bin/geesefs && echo user_allow_other | sudo tee -a /etc/fuse.conf'"
```
**Langfristig:** Packer Image mit geesefs vorinstalliert

---

### Cinder CSI CrashLoopBackOff
**Symptom:** `openstack-cinder-csi-controllerplugin` crasht nach Deploy
**Ursache:** EVS API Auth mit AK/SK nicht vollständig kompatibel — Keystone Token Auth benötigt
**Auswirkung:** EVS PVC (`demo-evs-pvc`) bleibt `Pending`
**Workaround:** `csi-cinder-sc-delete` StorageClass vorhanden aber nicht funktionsfähig ohne Keystone-kompatiblen Endpoint
**Status:** Offen — braucht OTC Keystone URL Investigation

---

### Terraform Provider `opentelekomcloud ~> 1.36`
**Einschränkung:** `opentelekomcloud_lb_loadbalancer_v3` hat keine `availability_zone` (singular) — muss `availability_zones` (plural) sein
**Fix:** Im Modul `shared-elb` korrekt als `availability_zones = [var.availability_zone]`

---

### OBS curl auf Nodes
**Problem:** `curl --aws-sigv4` auf Nodes gibt 400-Fehler wegen fehlendem `x-amz-content-sha256: UNSIGNED-PAYLOAD` Header
**Fix:** Header explizit mitgeben — oder SCP-Fallback via Bastion verwenden

---

### ingress-nginx `nginx-public` ELB bleibt beim Destroy
**Problem:** ingress-nginx deployed `LoadBalancer` Service → neuer CCM ELB → Pre-Destroy Script kennt ihn nicht
**Fix:** Pre-Destroy löscht alle ELBs mit `name.startswith('k8s-kubernetes-')` — deckt alle CCM-managed ELBs ab
**Achtung:** Manuell erstellte ELBs mit `k8s-kubernetes-` Prefix würden auch gelöscht werden

---

## 🟢 Bekannte Besonderheiten

### SSH Tunnel für kubectl
GitHub Actions Runner hat keinen direkten Zugriff auf `10.0.x.x:6443`. Pipeline nutzt:
```
localhost:6443 → SSH Tunnel via Bastion → Master:6443
```

### CCM Naming Convention
CCM benennt ELBs nach Schema: `k8s-kubernetes-<namespace>-<service-name>-<hash>`
Beispiel: `k8s-kubernetes-default-demo-lb-a1b2c3d4`

### OBS Endpoint (Swiss OTC)
Einziger funktionierender Endpoint für eu-ch2:
```
https://obs.eu-ch2.sc.otc.t-systems.com
```
(Andere Endpoints wie `s3.eu-ch2.otc.t-systems.com` funktionieren nicht zuverlässig)

### Terraform State Backend (OBS)
```hcl
bucket   = "rke2-sotc-tfstate"
key      = "rke2/terraform.tfstate"
region   = "eu-ch2"
endpoint = "https://obs.eu-ch2.sc.otc.t-systems.com"
```
Bucket muss **vor** erstem Apply existieren (wird nicht von Terraform erstellt).

### RKE2 Version
Fixiert auf `v1.34.5+rke2r1` — Upgrade erfordert neues cloud-init + rolling restart.

### ELB Health Check SNAT
OTC Dedicated ELB nutzt SNAT-Range `100.125.0.0/16` für Health Checks.
**Ohne** Security Group Rule für diese Range → NodePort Health Checks schlagen fehl → Pods `Unhealthy`.
Security Group Regel ist in `modules/networking/main.tf` vorhanden (`elb_snat_tcp` + `elb_snat_udp`).
