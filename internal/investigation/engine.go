package investigation

import (
	"context"

	"github.com/arruko/torvyn-sentientd/internal/argo"
	"github.com/arruko/torvyn-sentientd/internal/kagent"
	"github.com/arruko/torvyn-sentientd/internal/logging"
	"github.com/arruko/torvyn-sentientd/internal/policy"
	"github.com/arruko/torvyn-sentientd/internal/slack"
	"github.com/arruko/torvyn-sentientd/internal/store"
)

type Engine struct {
	store        store.IncidentStore
	kagentClient *kagent.Client
	argoClient   *argo.Client
	slackClient  *slack.Client
	policyEngine *policy.Engine
}

func NewEngine(store store.IncidentStore, k *kagent.Client, a *argo.Client, s *slack.Client, p *policy.Engine) *Engine {
	return &Engine{
		store:        store,
		kagentClient: k,
		argoClient:   a,
		slackClient:  s,
		policyEngine: p,
	}
}

// In v1, we'll call this from some simple trigger (e.g. periodic scan of new incidents).
// Later you can use watch/eventing.
func (e *Engine) InvestigateIncident(ctx context.Context, incID string) error {
	inc, err := e.store.GetByID(ctx, incID)
	if err != nil {
		return err
	}
	if inc == nil {
		return nil
	}

	logging.Info.Printf("Investigating incident %s", incID)

	req := kagent.InvestigationAndPlanRequest{
		Incident: *inc,
		Topology: inc.TopologyContext,
		Policies: map[string]any{
			"environment":            inc.Environment,
			"allowDirectK8sMutation": false,
			"allowedTools":           []string{"github.createPullRequest", "github.waitForMerge", "ci.waitForPipeline", "argoCD.waitForSync", "observability.checkForDuration", "k8s.getResource", "k8s.listPods", "prom.query", "grafana.query", "alertmanager.query"},
			"forbiddenTools":         []string{"k8s.applyManifest", "k8s.patchResource", "k8s.exec"},
		},
		InvestigationConfig: kagent.InvestigationConfig{
			MaxSteps:          12,
			MaxDurationSec:    600,
			MinRCAConfidence:  0.6,
			MinPlanConfidence: 0.6,
		},
	}

	result, err := e.kagentClient.InvestigateAndPlan(ctx, req)
	if err != nil {
		logging.Error.Printf("kagent investigation failed for incident %s: %v", incID, err)
		return err
	}

	// TODO: persist InvestigationResult if desired.

	// Validate DAG with policy engine before creating workflow
	if e.policyEngine != nil {
		logging.Info.Printf("Validating DAG with policy engine for incident %s", incID)
		if err := e.policyEngine.ValidateDAG(ctx, *inc, result.DAG); err != nil {
			logging.Error.Printf("⛔ POLICY REJECTED DAG for incident %s: %v", incID, err)

			// Notify Slack that policy rejected the plan
			if e.slackClient != nil {
				// Send RCA summary
				_ = e.slackClient.PostRCA(ctx, *inc, result.RCA, result.PlanConfidence, true)

				// Send specific policy rejection notification
				policyMsg := "⛔ *Remediation Plan Rejected by Policy*\n\n" +
					"Incident: " + incID + "\n" +
					"Service: " + inc.Service + "\n" +
					"Environment: " + inc.Environment + "\n\n" +
					"*Reason*: " + err.Error() + "\n\n" +
					"The proposed remediation plan violated safety policies and was NOT executed. " +
					"Manual intervention may be required."

				_ = e.slackClient.PostMessage(ctx, policyMsg)
			}

			// Do NOT create workflow - policy denied
			return err
		}
		logging.Info.Printf("✅ Policy approved DAG for incident %s", incID)
	} else {
		logging.Info.Printf("⚠️  Policy engine not configured, skipping DAG validation for incident %s", incID)
	}

	// Notify Slack with RCA summary
	if e.slackClient != nil {
		_ = e.slackClient.PostRCA(ctx, *inc, result.RCA, result.PlanConfidence, result.HumanApprovalNeeded)
	}

	// If plan confidence is good, create Argo workflow
	if result.PlanConfidence >= 0.6 {
		logging.Info.Printf("Creating Argo Workflow for incident %s (confidence: %.2f)", incID, result.PlanConfidence)
		if err := e.argoClient.CreateWorkflowForDAG(ctx, result.DAG); err != nil {
			logging.Error.Printf("Failed to create workflow for incident %s: %v", incID, err)

			// Notify Slack of workflow creation failure
			if e.slackClient != nil {
				failureMsg := "❌ *Workflow Creation Failed*\n\n" +
					"Incident: " + incID + "\n" +
					"Error: " + err.Error()
				_ = e.slackClient.PostMessage(ctx, failureMsg)
			}

			return err
		}
		logging.Info.Printf("✅ Argo Workflow created successfully for incident %s", incID)
	} else {
		logging.Info.Printf("⚠️  Plan confidence too low (%.2f < 0.6), skipping workflow creation for incident %s",
			result.PlanConfidence, incID)
	}

	return nil
}
