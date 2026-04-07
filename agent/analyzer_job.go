package main

import (
	"context"
	"fmt"

	"github.com/robfig/cron/v3"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobAnalyzer checks for Job failure conditions.
type JobAnalyzer struct{}

func (j *JobAnalyzer) Name() string { return "Job" }

func (j *JobAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	jobs, err := client.Clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, job := range jobs.Items {
		var failures []string

		// Check for failed conditions
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				failures = append(failures, fmt.Sprintf(
					"Job %s/%s has failed: %s - %s",
					job.Namespace, job.Name, cond.Reason, cond.Message))
			}
		}

		// Check for backoff limit exceeded
		if job.Status.Failed > 0 {
			if job.Spec.BackoffLimit != nil && job.Status.Failed >= *job.Spec.BackoffLimit {
				failures = append(failures, fmt.Sprintf(
					"Job %s/%s has reached backoff limit (%d failures)",
					job.Namespace, job.Name, job.Status.Failed))
			}
		}

		// Check for suspended jobs
		if job.Spec.Suspend != nil && *job.Spec.Suspend {
			failures = append(failures, fmt.Sprintf(
				"Job %s/%s is suspended",
				job.Namespace, job.Name))
		}

		// Check for deadline exceeded
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailureTarget && cond.Status == "True" {
				failures = append(failures, fmt.Sprintf(
					"Job %s/%s has failure target condition: %s",
					job.Namespace, job.Name, cond.Message))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Job",
				Name:  fmt.Sprintf("%s/%s", job.Namespace, job.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// CronJobAnalyzer checks for CronJob issues.
type CronJobAnalyzer struct{}

func (c *CronJobAnalyzer) Name() string { return "CronJob" }

func (c *CronJobAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	cronJobs, err := client.Clientset.BatchV1().CronJobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, cj := range cronJobs.Items {
		var failures []string

		// Check if the cronjob is suspended
		if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
			failures = append(failures, fmt.Sprintf(
				"CronJob %s/%s is suspended",
				cj.Namespace, cj.Name))
		}

		// Validate cron expression format
		schedule := cj.Spec.Schedule
		if schedule != "" {
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
			if _, err := parser.Parse(schedule); err != nil {
				failures = append(failures, fmt.Sprintf(
					"CronJob %s/%s has invalid cron expression %q: %v",
					cj.Namespace, cj.Name, schedule, err))
			}
		}

		// Check for too many active jobs (possible stuck jobs)
		if len(cj.Status.Active) > 0 {
			concurrencyPolicy := cj.Spec.ConcurrencyPolicy
			if concurrencyPolicy == batchv1.ForbidConcurrent && len(cj.Status.Active) > 1 {
				failures = append(failures, fmt.Sprintf(
					"CronJob %s/%s has %d active jobs but concurrency policy is Forbid",
					cj.Namespace, cj.Name, len(cj.Status.Active)))
			}
		}

		// Check for last schedule time
		isSuspended := cj.Spec.Suspend != nil && *cj.Spec.Suspend
		if cj.Status.LastScheduleTime == nil && !isSuspended {
			failures = append(failures, fmt.Sprintf(
				"CronJob %s/%s has never been scheduled",
				cj.Namespace, cj.Name))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "CronJob",
				Name:  fmt.Sprintf("%s/%s", cj.Namespace, cj.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}
