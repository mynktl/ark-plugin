#!/bin/bash

set -f
OPENEBS_NS=openebs
vp=$(kubectl get pods -n openebs -lopenebs.io/target=cstor-target -ocustom-columns='Volume:metadata.labels.openebs\.io/persistent-volume,Name:.metadata.name' --no-headers=true)
lastsnap=$(kubectl get cstorcompletedbackups -A -o=custom-columns=SNAP:.spec.snapName)
INVALID_VOL=`mktemp`
INVALID_CBKP=`mktemp`
INCREMENTAL_SNAP=`mktemp`
INCREMENTAL_CBKP=`mktemp`

function should_delete_snap(){
	snap=$1
	while read pline; do
		if [[ -z "$pline" ]]; then
			continue
		fi
		if [[ "$snap" == "$pline" ]]; then
			return 1
		fi
	done < <(echo "$lastsnap")
	return 0
}

function get_pod() {
	vol=$1

	while read pline; do
		read -r volume pod <<<$(echo $pline)
		if [ "$volume" == "$vol" ]; then
			echo $pod
			return
		fi
	done < <(echo "$vp")
}

function delete_snapshot() {
	vol="$1"
	snap="$2"

	if [[ -z "$vol" || -z "$snap" ]]; then
		echo "INVALID SNAPSHOT ==> volume:$vol snapshot:$snap"
		return
	fi

	pod=$(get_pod $vol)
	if [[ -z "$pod" ]]; then
		cat $INVALID_VOL |grep $vol
		[[ $? -eq 1 ]] && (echo $vol >> $INVALID_VOL)
		echo "INVALID VOLUME ==> volume:$vol snapshot:$snap"
		return
	fi


	out=$(kubectl exec ${pod} -n ${OPENEBS_NS} -ccstor-istgt -- istgtcontrol snapdestroy $vol $snap)
	ret=$?
	if [[ $ret -ne 0 ]]; then
		echo "ERROR SNAP DELETE ==> volume:$vol snapshot:$snap"
	fi
}

#readarray cbkp < <(kubectl get cstorbackups -A -o=custom-columns=Namespace:.metadata.namespace,Name:.metadata.name,Volume:.spec.volumeName,Snapshot:.spec.snapName --no-headers=true)
cbkp=$(kubectl get cstorbackups -A -o=custom-columns=Namespace:.metadata.namespace,Name:.metadata.name,Volume:.spec.volumeName,Snapshot:.spec.snapName --no-headers=true)

while read line; do
	bkp=($line)
	read -r ns cbname cbvol cbsnap <<<$(echo $line)
	if [[ -z "$ns" || -z "$cbname" || -z "$cbvol" || -z "$cbsnap" ]]; then
		echo "INVALID CSTORBACKUP ==> ns:$ns name:$cbname volume:$cbvol snapshot:$cbsnap"
		continue
	fi
	echo "DELETING SNAPSHOT ==> volume:$cbvol snapshot:$cbsnap"
	should_delete_snap $cbsnap
	dsnap=$?

	if [ $dsnap -eq 0 ]; then
		out=`delete_snapshot $cbvol $cbsnap`
		echo $out | grep "INVALID\|ERROR"
		ret=$?
		if [ $ret -ne 0 ]; then
			echo "DELETING CSTORBACKUP ==> ns:$ns name:$cbname"
			kubectl delete cstorbackup $cbname -n $ns
		else
			echo $out | grep "INVALID"
			if [ $? -eq 0 ]; then
				echo "kubectl delete cstorbackup $cbname -n $ns" >> $INVALID_CBKP
			else
				echo $out
			fi
		fi
	else
		echo "$cbname -n $ns" >> $INCREMENTAL_CBKP
		echo "$cbsnap volume:$cbvol cstorbackup -- $cbname -n $ns" >> $INCREMENTAL_SNAP
	fi
done < <(echo "$cbkp")

l=`cat $INVALID_VOL | wc -l`
if [[ $l -ne 0 ]]; then
	echo "==================================================="
	echo "Following volumes seems stale volume"
	cat $INVALID_VOL
fi


l=`cat $INVALID_CBKP | wc -l`
if [[ $l -ne 0 ]]; then
	echo "==================================================="
	echo "Following cstorbackup can be deleted manually"
	cat $INVALID_CBKP
fi

l=`cat $INCREMENTAL_SNAP | wc -l`
if [[ $l -ne 0 ]]; then
	echo "==================================================="
	echo "Following snapshots being used for incremental backup, so not deleted"
	cat $INCREMENTAL_SNAP
	echo "Relevant cstorbackups are as below"
	cat $INCREMENTAL_CBKP
fi
