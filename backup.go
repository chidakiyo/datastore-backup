package datastore_backup

import (
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/taskqueue"
	"net/http"
	u "net/url"
	"os"
	"strings"
)

func init() {
	http.HandleFunc("/backup", handler)
}

const (
	BackupQueueName = "backupQueue"
	BackupPath      = "/_ah/datastore_admin/backup.create"
)

func handler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	bucketName := os.Getenv("TARGET_BUCKET_NAME")
	backupPrefix := os.Getenv("BACKUP_PREFIX")
	ignoreKindsString := os.Getenv("IGNORE_KINDS")

	kinds, err := getKinds(c)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
		return
	}

	q := u.Values{
		"name":           {fmt.Sprintf("%s-", backupPrefix)},
		"filesystem":     {"gs"},
		"gs_bucket_name": {bucketName},
	}

	ignoreMap := map[string]bool{}
	if ignoreKindsString != "" {
		ignoreKinds := strings.Split(ignoreKindsString, ",")
		for _, kind := range ignoreKinds {
			ignoreMap[kind] = true
		}
	}

	for _, kind := range kinds {
		if _, ok := ignoreMap[kind]; !ok {
			q.Add("kind", kind)
		}
	}

	backupTask := taskqueue.NewPOSTTask(BackupPath, q)

	if _, err := taskqueue.Add(c, backupTask, BackupQueueName); err != nil {
		// fail
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
	} else {
		// success
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, http.StatusText(http.StatusOK))
	}
}

func getKinds(c context.Context) ([]string, error) {
	t := datastore.NewQuery("__kind__").KeysOnly().Run(c)
	var kinds []string
	for {
		key, err := t.Next(nil)
		if err == datastore.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.HasPrefix(key.StringID(), "_") {
			continue
		}
		kinds = append(kinds, key.StringID())
	}
	return kinds, nil
}
