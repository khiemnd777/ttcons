import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const featureFiles = [
  '../src/modules/projects/ProjectsPage.tsx',
  '../src/modules/projects/projectFormFields.ts',
  '../src/modules/warehouse/WarehousePage.tsx',
  '../src/modules/procurement/ProcurementPage.tsx',
  '../src/modules/rework/ReworkPage.tsx',
  '../src/modules/operations/ConstructionVariationProposalDialog.tsx',
  '../src/modules/boq/BOQPage.tsx',
  '../src/modules/boq/SupplierQuotationWorkspace.tsx',
  '../src/modules/contracts/ContractsPage.tsx',
  '../src/modules/project-finance/ProjectFinancePage.tsx',
  '../src/modules/accounting/AccountingCreateDialog.tsx',
  '../src/modules/accounting/TaxWorkspace.tsx',
  '../src/modules/accounting/ManualJournalDialog.tsx',
  '../src/modules/debt/InputInvoicePanel.tsx',
  '../src/modules/debt/OutputInvoicePanel.tsx',
  '../src/modules/debt/DebtCreateDialog.tsx',
  '../src/modules/debt/InputPaymentPlanPanel.tsx',
  '../src/modules/debt/OutputBillingPlanPanel.tsx',
];

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

function occurrences(value: string, pattern: RegExp) {
  return [...value.matchAll(pattern)].length;
}

test('the confirmed direct monetary inventory uses semantic currency controls', async () => {
  const sources = await Promise.all(featureFiles.map(source));
  const combined = sources.join('\n');
  assert.ok(occurrences(combined, /type:\s*'currency'/g) >= 30, 'expected the 30 SimpleForm monetary definitions to use currency');
  assert.equal(occurrences(combined, /type="currency"/g), 6, 'input/output invoice money fields must use their CurrencyInput branch');

  const boq = await source('../src/modules/boq/BOQPage.tsx');
  const supplierQuotes = await source('../src/modules/boq/SupplierQuotationWorkspace.tsx');
  const projectFinance = await source('../src/modules/project-finance/ProjectFinancePage.tsx');
  const manualJournal = await source('../src/modules/accounting/ManualJournalDialog.tsx');
  const constructionVariation = await source('../src/modules/operations/ConstructionVariationProposalDialog.tsx');
  assert.equal(occurrences(boq, /<PriceStage/g), 3);
  assert.equal(occurrences(supplierQuotes, /<CurrencyMiniInput/g), 2);
  assert.equal(occurrences(projectFinance, /<CurrencyInput/g), 3);
  assert.equal(occurrences(manualJournal, /<CurrencyInput/g), 4, 'debit and credit are rendered once per responsive layout');
  assert.equal(occurrences(constructionVariation, /<CurrencyInput/g), 1);
});

test('known monetary keys cannot regress to a generic number field', async () => {
  const combined = (await Promise.all(featureFiles.map(source))).join('\n');
  const monetaryKeys = '(?:estimated_budget|contract_amount|standard_price|total_estimated_amount|budget_impact|expected_unit_price|unit_price|discount_amount|original_amount|gross_amount|amount_delta|amount_before_vat|vat_amount|planned_amount|retention_amount|deduction_amount|taxable_delta|vat_delta|debit_amount|credit_amount)';
  assert.doesNotMatch(combined, new RegExp(`name:\\s*['"]${monetaryKeys}['"][^\\n]{0,220}type:\\s*['"]number['"]`, 'i'));
  assert.doesNotMatch(combined, /label="(?:Trước VAT|VAT|Tổng tiền|Tiền trước VAT \*|Tiền VAT|Số tiền \*|Giá trị ảnh hưởng dự kiến|Phát sinh Nợ|Phát sinh Có)"[^\n]{0,300}type="number"/i);
});
