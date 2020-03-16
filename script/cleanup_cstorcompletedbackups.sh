#!/bin/bash

set -f
OPENEBS_NS=openebs
vp=$(kubectl get cstorcompletedbackups -o=custom-columns=Namespace:.metadata.namespace,Name:.metadata.name -A  --no-headers=true)

while IFS= read pline; do
	read -r ns name <<<$(echo $pline)
	kubectl delete cstorcompletedbackups $name -n $ns
done < <(echo "$vp")

