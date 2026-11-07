import { toolOf } from './tools';

describe('usage', () => {
  it('keeps only well-known program names', () => {
    expect(toolOf('kubectl get pods -n shop')).toBe('kubectl');
    expect(toolOf('sudo -u postgres psql -c "select 1"')).toBe('psql');
    expect(toolOf('AWS_PROFILE=prod aws s3 ls')).toBe('aws');
    expect(toolOf('/usr/bin/systemctl restart nginx')).toBe('systemctl');
    expect(toolOf('env FOO=1 time docker ps')).toBe('docker');
    expect(toolOf('./deploy.sh --token=s3cret')).toBeNull();
    expect(toolOf('mysecretscript web-01.prod')).toBeNull();
    expect(toolOf('')).toBeNull();
  });
});
