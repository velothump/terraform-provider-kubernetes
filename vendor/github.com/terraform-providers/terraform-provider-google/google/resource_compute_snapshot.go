package google

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

func resourceComputeSnapshot() *schema.Resource {
	return &schema.Resource{
		Create: resourceComputeSnapshotCreate,
		Read:   resourceComputeSnapshotRead,
		Delete: resourceComputeSnapshotDelete,
		Exists: resourceComputeSnapshotExists,
		Update: resourceComputeSnapshotUpdate,

		Schema: map[string]*schema.Schema{
			"name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"zone": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"snapshot_encryption_key_raw": &schema.Schema{
				Type:      schema.TypeString,
				Optional:  true,
				ForceNew:  true,
				Sensitive: true,
			},

			"snapshot_encryption_key_sha256": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"source_disk_encryption_key_raw": &schema.Schema{
				Type:      schema.TypeString,
				Optional:  true,
				ForceNew:  true,
				Sensitive: true,
			},

			"source_disk_encryption_key_sha256": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"source_disk": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"source_disk_link": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"project": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},

			"self_link": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},

			"labels": &schema.Schema{
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"label_fingerprint": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceComputeSnapshotCreate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	// Build the snapshot parameter
	snapshot := &compute.Snapshot{
		Name: d.Get("name").(string),
	}

	source_disk := d.Get("source_disk").(string)

	if v, ok := d.GetOk("snapshot_encryption_key_raw"); ok {
		snapshot.SnapshotEncryptionKey = &compute.CustomerEncryptionKey{}
		snapshot.SnapshotEncryptionKey.RawKey = v.(string)
	}

	if v, ok := d.GetOk("source_disk_encryption_key_raw"); ok {
		snapshot.SourceDiskEncryptionKey = &compute.CustomerEncryptionKey{}
		snapshot.SourceDiskEncryptionKey.RawKey = v.(string)
	}

	op, err := config.clientCompute.Disks.CreateSnapshot(
		project, d.Get("zone").(string), source_disk, snapshot).Do()
	if err != nil {
		return fmt.Errorf("Error creating snapshot: %s", err)
	}

	// It probably maybe worked, so store the ID now
	d.SetId(snapshot.Name)

	err = computeOperationWait(config.clientCompute, op, project, "Creating Snapshot")
	if err != nil {
		return err
	}

	// Now if labels are set, go ahead and apply them
	if labels := expandLabels(d); len(labels) > 0 {
		// First, read the remote resource in order to find the fingerprint
		apiSnapshot, err := config.clientCompute.Snapshots.Get(project, d.Id()).Do()
		if err != nil {
			return fmt.Errorf("Eror when reading snapshot for label update: %s", err)
		}

		err = updateLabels(config.clientCompute, project, d.Id(), labels, apiSnapshot.LabelFingerprint)
		if err != nil {
			return err
		}
	}
	return resourceComputeSnapshotRead(d, meta)
}

func resourceComputeSnapshotRead(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	snapshot, err := config.clientCompute.Snapshots.Get(
		project, d.Id()).Do()
	if err != nil {
		return handleNotFoundError(err, d, fmt.Sprintf("Snapshot %q", d.Get("name").(string)))
	}

	d.Set("self_link", snapshot.SelfLink)
	d.Set("source_disk_link", snapshot.SourceDisk)
	d.Set("name", snapshot.Name)

	if snapshot.SnapshotEncryptionKey != nil && snapshot.SnapshotEncryptionKey.Sha256 != "" {
		d.Set("snapshot_encryption_key_sha256", snapshot.SnapshotEncryptionKey.Sha256)
	}

	if snapshot.SourceDiskEncryptionKey != nil && snapshot.SourceDiskEncryptionKey.Sha256 != "" {
		d.Set("source_disk_encryption_key_sha256", snapshot.SourceDiskEncryptionKey.Sha256)
	}

	d.Set("labels", snapshot.Labels)
	d.Set("label_fingerprint", snapshot.LabelFingerprint)

	return nil
}

func resourceComputeSnapshotUpdate(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	d.Partial(true)

	if d.HasChange("labels") {
		err = updateLabels(config.clientCompute, project, d.Id(), expandLabels(d), d.Get("label_fingerprint").(string))
		if err != nil {
			return err
		}

		d.SetPartial("labels")
	}

	d.Partial(false)

	return resourceComputeSnapshotRead(d, meta)
}

func resourceComputeSnapshotDelete(d *schema.ResourceData, meta interface{}) error {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return err
	}

	// Delete the snapshot
	op, err := config.clientCompute.Snapshots.Delete(
		project, d.Id()).Do()
	if err != nil {
		if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == 404 {
			log.Printf("[WARN] Removing Snapshot %q because it's gone", d.Get("name").(string))
			// The resource doesn't exist anymore
			d.SetId("")
			return nil
		}
		return fmt.Errorf("Error deleting snapshot: %s", err)
	}

	err = computeOperationWait(config.clientCompute, op, project, "Deleting Snapshot")
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}

func resourceComputeSnapshotExists(d *schema.ResourceData, meta interface{}) (bool, error) {
	config := meta.(*Config)

	project, err := getProject(d, config)
	if err != nil {
		return false, err
	}

	_, err = config.clientCompute.Snapshots.Get(
		project, d.Id()).Do()
	if err != nil {
		if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == 404 {
			log.Printf("[WARN] Removing Snapshot %q because it's gone", d.Get("name").(string))
			// The resource doesn't exist anymore
			d.SetId("")

			return false, err
		}
		return true, err
	}
	return true, nil
}

func updateLabels(client *compute.Service, project string, resourceId string, labels map[string]string, labelFingerprint string) error {
	setLabelsReq := compute.GlobalSetLabelsRequest{
		Labels:           labels,
		LabelFingerprint: labelFingerprint,
	}
	op, err := client.Snapshots.SetLabels(project, resourceId, &setLabelsReq).Do()
	if err != nil {
		return err
	}

	return computeOperationWait(client, op, project, "Setting labels on snapshot")
}
