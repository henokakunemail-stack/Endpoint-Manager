import React, { useState } from 'react'
import {
  Database,
  FileSpreadsheet,
  FileText,
  HardDrive,
  History,
  Layers,
  Shield,
  ShieldCheck,
} from 'lucide-react'
import { api } from '../services/api'

interface ReportCardProps {
  title: string
  category: string
  description: string
  reportType: 'inventory' | 'patches' | 'deployments' | 'audit'
  icon: React.ReactNode
}

const ReportCard: React.FC<ReportCardProps> = ({ title, category, description, reportType, icon }) => {
  const [downloading, setDownloading] = useState(false)

  const handleDownload = (format: 'csv' | 'json') => {
    setDownloading(true)
    const url = api.getReportExportUrl(reportType, format)
    // Create an invisible link to trigger direct download
    const link = document.createElement('a')
    link.href = url
    link.setAttribute('download', `${reportType}-report.${format}`)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    setTimeout(() => setDownloading(false), 1500)
  }

  return (
    <div className="report-card">
      <div className="report-card-top">
        <div className="report-icon-box">{icon}</div>
        <div className="report-header-info">
          <span className="report-badge">{category}</span>
          <h3 className="report-title">{title}</h3>
        </div>
      </div>
      <p className="report-description">{description}</p>
      <div className="report-actions">
        <button
          type="button"
          className="btn btn-secondary btn-sm"
          onClick={() => handleDownload('csv')}
          disabled={downloading}
          title="Download as CSV spreadsheet"
        >
          <FileSpreadsheet size={15} className="text-success" />
          <span>Export CSV</span>
        </button>
        <button
          type="button"
          className="btn btn-secondary btn-sm"
          onClick={() => handleDownload('json')}
          disabled={downloading}
          title="Download as structured JSON"
        >
          <FileText size={15} className="text-primary" />
          <span>Export JSON</span>
        </button>
      </div>
    </div>
  )
}

export const ReportsPage: React.FC = () => {
  return (
    <div className="page-container">
      <div className="page-header">
        <div>
          <h2 className="page-title">Compliance Reports & Data Exports</h2>
          <p className="page-subtitle">
            Generate on-demand audit logs, hardware inventories, patch compliance manifests, and deployment statistics
          </p>
        </div>
      </div>

      {/* Info Banner */}
      <div className="alert-banner alert-success mb-6">
        <ShieldCheck size={18} />
        <div>
          <span className="font-semibold block">Production-Ready Streaming Exports</span>
          <span className="text-xs text-dim">
            All report endpoints stream records directly from the database using chunked HTTP transfer to support large fleet datasets with minimal memory overhead.
          </span>
        </div>
      </div>

      {/* Report Cards Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <ReportCard
          title="Hardware & System Inventory"
          category="Asset Management"
          description="Complete inventory of all enrolled branch endpoints, including CPU, RAM, disk partitions, network interfaces, serial numbers, and OS versions."
          reportType="inventory"
          icon={<HardDrive size={22} className="text-primary" />}
        />

        <ReportCard
          title="Fleet Patch & Vulnerability Report"
          category="Security & Compliance"
          description="Detailed manifest of installed and missing security updates, pending CVE hotfixes, reboot requirements, and overall fleet patch compliance rates."
          reportType="patches"
          icon={<Shield size={22} className="text-warning" />}
        />

        <ReportCard
          title="Software Deployment History"
          category="Operations"
          description="Full deployment lifecycle records, silent installer rollouts, target branch groups, return exit codes, and package distribution success rates."
          reportType="deployments"
          icon={<Layers size={22} className="text-success" />}
        />

        <ReportCard
          title="Security & System Audit Trail"
          category="Governance & Audit"
          description="Immutable security audit trail recording all administrator actions, remote command dispatches, credential changes, and system modifications."
          reportType="audit"
          icon={<History size={22} className="text-danger" />}
        />
      </div>

      {/* Audit Compliance Notes */}
      <div className="table-card mt-8 p-6">
        <div className="flex items-center gap-3 mb-3">
          <Database className="text-primary" size={20} />
          <h3 className="font-semibold text-main text-base">Corporate Audit Standard Compliance</h3>
        </div>
        <p className="text-sm text-muted leading-relaxed">
          Reports generated by this system comply with enterprise IT governance frameworks (ISO 27001, SOC 2, and internal corporate audit mandates).
          All exports include unique timestamps, machine identifiers, and cryptographic hash verification references.
        </p>
      </div>
    </div>
  )
}
