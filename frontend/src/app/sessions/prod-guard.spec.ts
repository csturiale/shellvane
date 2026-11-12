import { contextInCommand, isRiskyKubeCommand } from './prod-guard';

describe('prod guard', () => {
  it('finds the risky kubectl and helm commands', () => {
    for (const c of [
      'kubectl delete pod web-1',
      'kubectl -n argocd delete secret argocd-secret',
      'kubectl drain node-3 --ignore-daemonsets',
      'kubectl scale deploy/web --replicas=0',
      'kubectl rollout restart deploy/web',
      'kubectl apply -f x.yaml --prune -l app=web',
      'kubectl edit cm config',
      'helm uninstall argocd -n argocd',
      'helm upgrade --install argocd ./chart',
    ]) {
      expect([c, isRiskyKubeCommand(c)]).toEqual([c, true]);
    }
    for (const c of ['kubectl get pods', 'kubectl describe pod web', 'kubectl logs -f web', 'kubectl scale deploy/web --replicas=3', 'helm list -A', 'echo delete', 'kubectl apply -f x.yaml']) {
      expect([c, isRiskyKubeCommand(c)]).toEqual([c, false]);
    }
  });

  it('reads the context named in the command', () => {
    expect(contextInCommand('kubectl --context shop-prd delete pod x')).toBe('shop-prd');
    expect(contextInCommand('kubectl delete pod x --context=shop-qat')).toBe('shop-qat');
    expect(contextInCommand('helm --kube-context "pay-prd" uninstall a')).toBe('pay-prd');
    expect(contextInCommand('kubectl delete pod x')).toBe('');
  });
});
